Vagrant.configure("2") do |config|
  boxes = {
    "ubuntu-node-1" => { box: "bento/ubuntu-24.04" },
    "ubuntu-node-2" => { box: "bento/ubuntu-24.04" }
  }

  boxes.each do |name, cfg|
    config.vm.define name do |node|
      node.vm.box = cfg[:box]
      node.vm.network "public_network", use_dhcp_assigned_default_route: true
      node.vm.box_version = "202510.26.0"

      node.vm.provider "virtualbox" do |vb|
        vb.memory = "8192"
        vb.cpus = 4
        vb.name = "servehub-test-#{name}"
        vb.customize ["modifyvm", :id, "--graphicscontroller", "vmsvga"]
      end
    end
  end
end