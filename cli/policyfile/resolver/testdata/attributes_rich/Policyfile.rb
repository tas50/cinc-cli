name 'attributes_rich'
run_list 'base'
default['zeta'] = 'last'
default['alpha']['enabled'] = true
default['alpha']['count'] = 3
default['alpha']['names'] = ['x', 'y', 'z']
default['middle']['nested']['deep'] = 'value'
override['security']['level'] = 5
override['security']['flags'] = ['a', 'b']
cookbook 'base', path: 'cookbooks/base'
