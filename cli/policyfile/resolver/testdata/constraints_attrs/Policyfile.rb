name 'constraints_attrs'
run_list 'app'
named_run_list :db, 'app::default'
default['app']['port'] = 8080
override['app']['debug'] = true
cookbook 'app', '~> 2.0', path: 'cookbooks/app'
cookbook 'lib', path: 'cookbooks/lib'
