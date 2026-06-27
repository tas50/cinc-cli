name 'dyn_env'
run_list 'recipe[app::default]'
cookbook 'app', path: '.'
# Cookbook added conditionally on an environment variable (default keeps the
# golden deterministic; the Go test also flips this to prove ENV plumbing).
if ENV.fetch('WITH_MONITORING', '1') == '1'
  cookbook 'monitoring', '~> 2.0'
end
cookbook 'logging' if ENV['WITH_LOGGING'] == 'yes'
