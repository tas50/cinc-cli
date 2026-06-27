name "app-#{'prod'.upcase.downcase}"
app_version = '2.4'
run_list "recipe[app::v#{app_version.tr('.', '_')}]"
cookbook 'app', "~> #{app_version}"
