name 'cb_path'
run_list 'recipe[app::default]'
cookbook 'app', path: '.'
cookbook 'shared', path: '../shared'
