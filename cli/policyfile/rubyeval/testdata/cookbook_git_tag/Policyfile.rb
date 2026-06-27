name 'cb_git_tag'
run_list 'recipe[app::default]'
cookbook 'app', '~> 3.0', git: 'https://github.com/example/app', tag: 'release-3.0'
