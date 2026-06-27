# frozen_string_literal: false
#
# cinc Policyfile DSL shim.
#
# This file runs INSIDE ruby.wasm (real CRuby), under wazero. It defines a
# faithful reimplementation of ChefCLI::Policyfile::DSL (chef-cli's Policyfile
# evaluation surface) and then `instance_eval`s the user's Policyfile.rb against
# an instance of it. Because the user's file runs in a real Ruby VM, ANY valid
# Ruby works (loops, conditionals, helper methods, ENV, string interpolation,
# stdlib, require_relative against the preopened work dir). Only the Policyfile
# *DSL methods* are this shim; everything else is genuine Ruby.
#
# The captured declarations are serialized to a canonical JSON document and
# written to ARGV[1]. The Go side (cli/policyfile/rubyeval) unmarshals it into
# an EvaluatedPolicy. The canonical schema and the chef semantics it mirrors are
# documented in doc.go and mirrored by generate_goldens.rb (which drives the
# REAL chef-cli DSL), so the two must produce identical JSON for any fixture.
#
# Semantics are ported from chef-cli 6.1.34 lib/chef-cli/policyfile/dsl.rb.

require "json"

# Vendored, pure-Ruby semverse (Apache-2.0) gives byte-identical version
# constraint normalization to chef (which uses the same gem). ARGV[2] is the
# directory the engine wrote the vendored copy into.
$LOAD_PATH.unshift(ARGV[2]) if ARGV[2]
require "semverse"

module CincPolicyfile
  # RUN_LIST_ITEM_COMPONENT mirrors ChefCLI::Policyfile::DSL.
  RUN_LIST_ITEM_COMPONENT = /^[.[:alnum:]_-]+$/.freeze

  # COMMUNITY_DEFAULT_URI is chef's default supermarket/community source URI.
  COMMUNITY_DEFAULT_URI = "https://supermarket.chef.io".freeze

  # AttributeMash is an autovivifying, string-keyed hash that mirrors the
  # behavior of Chef::Node::Attribute's default/override VividMash closely
  # enough for Policyfile use: nested `default['a']['b'] = c` chains create
  # intermediate hashes, and all keys are stored as strings.
  class AttributeMash
    def initialize
      @store = {}
    end

    def [](key)
      key = convert_key(key)
      v = @store[key]
      if v.nil? && !@store.key?(key)
        v = AttributeMash.new
        @store[key] = v
      end
      v
    end

    def []=(key, value)
      @store[convert_key(key)] = value
    end

    # to_value renders the mash (and any nested mashes) as plain Ruby data for
    # JSON serialization. Empty autovivified branches collapse to {}.
    def to_value
      out = {}
      @store.each { |k, v| out[k] = convert_value(v) }
      out
    end

    private

    def convert_key(key)
      key.is_a?(Symbol) ? key.to_s : key
    end

    def convert_value(v)
      case v
      when AttributeMash then v.to_value
      when Hash
        h = {}
        v.each { |k, val| h[convert_key(k)] = convert_value(val) }
        h
      when Array then v.map { |e| convert_value(e) }
      else v
      end
    end
  end

  # CookbookSpec captures one `cookbook` declaration.
  CookbookSpec = Struct.new(:name, :version_constraint, :source_options)

  # IncludedPolicy captures one `include_policy` declaration.
  IncludedPolicy = Struct.new(:name, :source_options)

  # DSL is the receiver the user's Policyfile.rb is instance_eval'd against.
  class DSL
    attr_reader :errors, :cookbook_specs, :included_policies,
                :named_run_lists, :default_sources

    # policyfile_root is the notional directory the Policyfile lives in. It is
    # used only to expand `default_source :chef_repo` paths (chef expands those
    # against the Policyfile's directory). A fixed sentinel keeps goldens
    # portable; the Go engine and generate_goldens.rb pass the same value.
    def initialize(policyfile_root)
      @policyfile_root = policyfile_root
      @name = nil
      @errors = []
      @run_list = []
      @named_run_lists = {}
      @included_policies = []
      @cookbook_specs = {}
      @default_sources = []
      @default_attrs = AttributeMash.new
      @override_attrs = AttributeMash.new
    end

    def name(name = nil)
      @name = name unless name.nil?
      @name
    end

    def run_list(*items)
      items = items.flatten
      unless items.empty?
        validate_run_list_items(items)
        @run_list = items
      end
      @run_list
    end

    def named_run_list(name, *items)
      items = items.flatten
      unless items.empty?
        validate_run_list_items(items, name)
        @named_run_lists[name.to_s] = items
      end
      @named_run_lists[name.to_s]
    end

    def default_source(source_type = nil, source_argument = nil, &block)
      return @default_sources if source_type.nil?

      case source_type
      when :community, :supermarket
        @default_sources << { "type" => "community", "uri" => source_argument || COMMUNITY_DEFAULT_URI }
      when :delivery_supermarket
        if source_argument.nil?
          @errors << "You must specify the server's URI when using a default_source :delivery_supermarket"
        else
          @default_sources << { "type" => "delivery_supermarket", "uri" => source_argument }
        end
      when :chef_server
        if source_argument.nil?
          @errors << "You must specify the server's URI when using a default_source :chef_server"
        else
          @default_sources << { "type" => "chef_server", "uri" => source_argument }
        end
      when :chef_repo
        if source_argument.nil?
          @errors << "You must specify the path to the chef-repo when using a default_source :chef_repo"
        else
          @default_sources << { "type" => "chef_repo", "path" => File.expand_path(source_argument, @policyfile_root) }
        end
      when :artifactory
        if source_argument.nil?
          @errors << "You must specify the server's URI when using a default_source :artifactory"
        else
          @default_sources << { "type" => "artifactory", "uri" => source_argument }
        end
      else
        @errors << "Invalid default_source type '#{source_type.inspect}'"
      end
    end

    def cookbook(name, *version_and_source_opts)
      source_options =
        if version_and_source_opts.last.is_a?(Hash)
          version_and_source_opts.pop
        else
          {}
        end
      constraint = version_and_source_opts.first || ">= 0.0.0"
      normalized = Semverse::Constraint.new(constraint).to_s

      if @cookbook_specs[name]
        @errors << "Cookbook '#{name}' assigned to conflicting sources"
      else
        @cookbook_specs[name] = CookbookSpec.new(name, normalized, stringify_options(source_options))
      end
    end

    def include_policy(name, source_options = {})
      if @included_policies.any? { |p| p.name == name }
        @errors << "Included policy '#{name}' assigned conflicting locations or was already specified"
      else
        @included_policies << IncludedPolicy.new(name, stringify_options(source_options))
      end
    end

    def default
      @default_attrs
    end

    def override
      @override_attrs
    end

    # eval_policyfile mirrors chef's: instance_eval the source with a filename
    # (so backtraces and require_relative resolve), then validate. Exceptions do
    # not propagate; chef records them in @errors, and so do we.
    def eval_policyfile(policyfile_string, filename)
      instance_eval(policyfile_string, filename)
      validate!
      self
    rescue SyntaxError => e
      @errors << "Invalid Ruby syntax in Policyfile '#{filename}':\n\n#{e.message}"
      self
    rescue SignalException, SystemExit
      raise
    rescue Exception => e
      @errors << "Evaluation of policyfile '#{filename}' raised an exception\n  Exception: #{e.class.name} \"#{e}\"\n"
      self
    end

    def to_h
      {
        "name" => @name,
        "run_list" => @run_list,
        "named_run_lists" => @named_run_lists,
        "default_source" => @default_sources,
        "cookbooks" => cookbooks_hash,
        "included_policies" => @included_policies.map { |p| { "name" => p.name, "source_options" => p.source_options } },
        "default_attributes" => @default_attrs.to_value,
        "override_attributes" => @override_attrs.to_value,
        "errors" => @errors,
      }
    end

    private

    def cookbooks_hash
      out = {}
      @cookbook_specs.each do |n, spec|
        out[n] = { "version_constraint" => spec.version_constraint, "source_options" => spec.source_options }
      end
      out
    end

    def stringify_options(opts)
      out = {}
      opts.each { |k, v| out[k.is_a?(Symbol) ? k.to_s : k] = v }
      out
    end

    def validate!
      @errors << "Invalid run_list. run_list cannot be empty" if @run_list.empty?
    end

    def validate_run_list_items(items, run_list_name = nil)
      items.each do |item|
        run_list_desc = run_list_name.nil? ? "Run List Item '#{item}'" : "Named Run List '#{run_list_name}' Item '#{item}'"
        item_name = run_list_item_name(item)
        cookbook, separator, recipe = item_name.partition("::")

        if RUN_LIST_ITEM_COMPONENT.match(cookbook).nil?
          message = "#{run_list_desc} has invalid cookbook name '#{cookbook}'.\nCookbook names can only contain alphanumerics, hyphens, and underscores."
          message << "\nDid you mean '#{item.sub(":", "::")}'?" if /[^:]:[^:]/.match?(cookbook)
          @errors << message
        end
        unless separator.empty?
          if RUN_LIST_ITEM_COMPONENT.match(recipe).nil?
            @errors << "#{run_list_desc} has invalid recipe name '#{recipe}'.\nRecipe names can only contain alphanumerics, hyphens, and underscores."
          end
        end
      end
    end

    # run_list_item_name extracts the bare name from a run_list item, mirroring
    # Chef::RunList::RunListItem#name: strips a recipe[...] / role[...] wrapper
    # and any @version suffix.
    def run_list_item_name(item)
      s = item.to_s
      if (m = /\A(recipe|role)\[(.+)\]\z/.match(s))
        s = m[2]
      end
      s = s.split("@", 2).first if s.include?("@")
      s
    end
  end
end

# --- entry point -------------------------------------------------------------
# ARGV[0]: path to the user's Policyfile.rb (inside the preopened work dir)
# ARGV[1]: path to write the canonical JSON result to
# ARGV[2]: directory holding the vendored semverse library
# ARGV[3]: policyfile_root sentinel for chef_repo path expansion
policyfile_path = ARGV[0]
output_path = ARGV[1]
policyfile_root = ARGV[3] || "/cinc/policyfile"

source = File.read(policyfile_path)
dsl = CincPolicyfile::DSL.new(policyfile_root)
dsl.eval_policyfile(source, policyfile_path)

File.write(output_path, JSON.generate(dsl.to_h))
