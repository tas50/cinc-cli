name 'dyn_loop'
# run_list built dynamically from a loop — the whole reason for real Ruby.
run_list(%w[web app db].map { |tier| "role[#{tier}]" })
%w[base monitoring].each do |cb|
  cookbook cb
end
