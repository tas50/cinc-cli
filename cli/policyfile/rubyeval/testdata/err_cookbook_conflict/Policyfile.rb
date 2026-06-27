name 'err_conflict'
run_list 'recipe[app::default]'
cookbook 'app', path: './app'
cookbook 'app', git: 'https://github.com/example/app'
