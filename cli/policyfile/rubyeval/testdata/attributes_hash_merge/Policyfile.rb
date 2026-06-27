name 'attr_merge'
run_list 'recipe[app::default]'
base = { 'a' => 1, 'b' => { 'c' => 2 } }
extra = { 'b' => { 'd' => 3 }, 'e' => 4 }
default['settings'] = base.merge(extra)
