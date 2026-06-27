name 'cb_range'
run_list 'recipe[app::default]'
# chef keeps only the FIRST constraint; '< 3.0' is intentionally dropped.
cookbook 'app', '>= 1.0', '< 3.0'
