Vagrant.configure("2") do |config|
  boxes = {
    "arch"    => { box: "archlinux/archlinux", ip: "192.168.56.10" },
    "debian"  => { box: "debian/bookworm64", ip: "192.168.56.11" },
    "ubuntu"  => { box: "ubuntu/jammy64", ip: "192.168.56.12" }
  }

  boxes.each do |name, cfg|
    config.vm.define name do |node|
      node.vm.box = cfg[:box]
      node.vm.network "private_network", ip: cfg[:ip]

      node.disksize.size = '40GB'
      
      node.vm.provider "virtualbox" do |vb|
        vb.memory = "1024"
        vb.cpus = 1
        vb.name = "servehub-test-#{name}"
      end
    end
  end
end