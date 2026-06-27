name 'dyn_require'
require_relative 'cookbooks'
run_list 'recipe[app::default]'
EXTRA_COOKBOOKS.each { |cb| cookbook cb }
