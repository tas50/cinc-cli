name 'attr_simple'
run_list 'recipe[app::default]'
default['app']['port'] = 8080
default['app']['enabled'] = true
override['app']['debug'] = false
