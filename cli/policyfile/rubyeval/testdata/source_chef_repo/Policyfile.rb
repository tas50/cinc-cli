name 'src_chef_repo'
default_source :chef_repo, '../cookbooks'
run_list 'recipe[app::default]'
