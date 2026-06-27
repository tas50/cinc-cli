name 'metadata_json'
run_list 'web'
cookbook 'web', path: 'cookbooks/web'
cookbook 'util', path: 'cookbooks/util'
