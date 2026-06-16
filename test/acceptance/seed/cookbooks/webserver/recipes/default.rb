#
# Cookbook:: webserver
# Recipe:: default
#

package 'nginx'

service 'nginx' do
  action %i(enable start)
end
