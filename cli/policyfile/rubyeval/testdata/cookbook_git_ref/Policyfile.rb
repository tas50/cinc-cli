name 'cb_git_ref'
run_list 'recipe[app::default]'
cookbook 'app', git: 'https://github.com/example/app', ref: 'v1.2.3'
