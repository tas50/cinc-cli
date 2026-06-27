name 'dyn_helper'
# A helper method defined in the Policyfile and called from it.
def supermarket_cookbook(cb, version)
  cookbook cb, "~> #{version}"
end
run_list 'recipe[app::default]'
supermarket_cookbook 'apache2', '5.0'
supermarket_cookbook 'mysql', '8.0'
