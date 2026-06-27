name 'src_chef_server'
default_source :chef_server, 'https://chef.example.com/organizations/acme'
run_list 'recipe[app::default]'
