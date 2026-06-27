name 'dyn_fileread'
version = File.read(File.join(__dir__, 'version.txt')).strip
run_list 'recipe[app::default]'
cookbook 'app', "= #{version}"
