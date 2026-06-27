name 'multi'
run_list 'recipe[app::default]', 'role[web]', 'recipe[app::db]'
