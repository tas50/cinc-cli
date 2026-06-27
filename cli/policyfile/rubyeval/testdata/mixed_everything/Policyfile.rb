name 'kitchen_sink'
default_source :supermarket
default_source :chef_server, 'https://chef.example.com/organizations/acme'
run_list 'recipe[app::default]', 'role[web]'
named_run_list :update, 'recipe[app::update]'
cookbook 'app', path: '.'
cookbook 'apache2', '~> 5.0'
cookbook 'mysql', '>= 8.0', '< 9.0'
cookbook 'nginx', git: 'https://github.com/example/nginx', ref: 'v1.0.0'
include_policy 'base', path: '../base/Policyfile.lock.json'
default['app']['port'] = 8080
default['app']['db']['host'] = 'localhost'
override['app']['env'] = 'production'
