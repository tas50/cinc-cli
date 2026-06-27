name 'dyn_hash'
run_list 'recipe[app::default]'
defaults = { 'pool_size' => 5, 'timeout' => 30 }
overrides = { 'timeout' => 60, 'ssl' => true }
config = defaults.merge(overrides)
config.each { |k, v| default['db'][k] = v }
