name 'named'
run_list 'recipe[app::default]'
named_run_list :update, 'recipe[app::update]'
named_run_list 'integration', 'recipe[app::default]', 'recipe[app::test]'
