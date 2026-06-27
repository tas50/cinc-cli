name 'cb_git_branch'
run_list 'recipe[app::default]'
cookbook 'app', git: 'https://github.com/example/app', branch: 'main'
