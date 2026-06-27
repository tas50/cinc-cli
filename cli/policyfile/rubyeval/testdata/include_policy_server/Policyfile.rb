name 'inc_server'
run_list 'recipe[app::default]'
include_policy 'base', server: 'https://chef.example.com', policy_name: 'base', policy_group: 'prod'
