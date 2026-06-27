name 'src_artifactory'
default_source :artifactory, 'https://artifactory.example.com/api/chef'
run_list 'recipe[app::default]'
