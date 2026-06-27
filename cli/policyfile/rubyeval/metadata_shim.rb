# frozen_string_literal: false
#
# cinc cookbook metadata.rb shim.
#
# Runs INSIDE ruby.wasm (real CRuby), under wazero, exactly like shim.rb but for
# a cookbook's metadata.rb instead of a Policyfile.rb. It reimplements just
# enough of Chef::Cookbook::Metadata's DSL to capture the three things the
# Policyfile resolver needs — name, version, and the ordered list of
# `depends` — and swallows every other metadata directive (maintainer, license,
# supports, attribute, gem, chef_version, ...) via method_missing, since they do
# not affect dependency resolution or the cookbook identifier.
#
# Because metadata.rb runs in genuine CRuby, any valid Ruby works: version
# helpers, require_relative of sibling files, conditionals, string
# interpolation. Dependency version constraints are normalized through the same
# vendored Semverse chef uses, so `depends 'x'` -> ">= 0.0.0" and `~> 1.0` stays
# "~> 1.0", byte-identical to chef.
#
# Output JSON schema (written to ARGV[1]):
#   { "name": String, "version": String,
#     "dependencies": [[name, constraint], ...], "errors": [String, ...] }

require "json"

# ARGV[2] is the directory the engine wrote the vendored semverse copy into.
$LOAD_PATH.unshift(ARGV[2]) if ARGV[2]
require "semverse"

module CincMetadata
  class DSL
    attr_reader :errors

    def initialize
      @name = nil
      @version = nil
      @deps = {}      # name => normalized constraint (insertion-ordered)
      @dep_order = [] # preserves first-seen order of dep names
      @errors = []
    end

    def eval_metadata(source, filename)
      instance_eval(source, filename, 1)
    rescue SyntaxError, StandardError => e
      @errors << "#{e.class}: #{e.message}"
    end

    # --- captured directives ---------------------------------------------------

    def name(arg = nil)
      @name = arg.to_s if arg
      @name
    end

    def version(arg = nil)
      @version = arg.to_s if arg
      @version
    end

    def depends(cookbook, *version_args)
      cookbook = cookbook.to_s
      constraint =
        if version_args.empty?
          ">= 0.0.0"
        else
          version_args.join(", ")
        end
      normalized = Semverse::Constraint.new(constraint).to_s
      unless @deps.key?(cookbook)
        @dep_order << cookbook
      end
      @deps[cookbook] = normalized
    rescue StandardError => e
      @errors << "invalid dependency '#{cookbook}': #{e.message}"
    end

    # Every other metadata directive (maintainer, license, supports, provides,
    # attribute, recipe, gem, chef_version, ohai_version, source_url, ...) is
    # irrelevant to resolution and the identifier, so swallow it.
    def method_missing(_name, *_args, &_blk)
      nil
    end

    def respond_to_missing?(_name, _include_private = false)
      true
    end

    def to_h
      {
        "name" => @name,
        "version" => @version,
        "dependencies" => @dep_order.map { |cb| [cb, @deps[cb]] },
        "errors" => @errors,
      }
    end
  end
end

# --- entry point -------------------------------------------------------------
# ARGV[0]: path to the cookbook's metadata.rb (inside the preopened work dir)
# ARGV[1]: path to write the canonical JSON result to
# ARGV[2]: directory holding the vendored semverse library
metadata_path = ARGV[0]
output_path = ARGV[1]

dsl = CincMetadata::DSL.new
dsl.eval_metadata(File.read(metadata_path), metadata_path)

File.write(output_path, JSON.generate(dsl.to_h))
