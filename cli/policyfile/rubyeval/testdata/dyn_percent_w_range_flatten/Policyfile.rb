name 'dyn_pw'
# %w[], ranges, and Array#flatten feeding the run_list.
recipes = %w[app::base app::config]
versions = (1..3).map { |n| "recipe[feature_#{n}]" }
run_list [recipes.map { |r| "recipe[#{r}]" }, versions].flatten
