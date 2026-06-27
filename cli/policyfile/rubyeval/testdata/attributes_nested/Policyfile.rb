name 'attr_nested'
run_list 'recipe[app::default]'
default['a']['b']['c']['d'] = 'deep'
override['x']['y'] = [1, 2, 3]
