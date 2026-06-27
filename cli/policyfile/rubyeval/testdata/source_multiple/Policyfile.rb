name 'src_multi'
default_source :supermarket
default_source :chef_server, 'https://chef.example.com/organizations/acme'
run_list 'recipe[app::default]'
