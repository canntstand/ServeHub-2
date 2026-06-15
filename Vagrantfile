Vagrant.configure("2") do |config|
  boxes = {
    "arch-node"    => { box: "generic/arch" },
    "debian-node"  => { box: "generic/debian12" },
    "ubuntu-node"  => { box: "generic/ubuntu2204" }
  }

  boxes.each do |name, cfg|
    config.vm.define name do |node|
      node.vm.box = cfg[:box]
      node.vm.network "public_network", use_dhcp_assigned_default_route: true
      
      if name == "arch-node"
        node.vm.provision "shell", inline: <<-SHELL
          echo "Обновление ключей PGP для Arch Linux..."
          pacman -Sy --noconfirm archlinux-keyring
          pacman -Su --noconfirm
        SHELL
      end
      
      node.vm.provider "virtualbox" do |vb|
        vb.memory = "4096"
        vb.cpus = 3
        vb.name = "servehub-test-#{name}"
        vb.customize ["modifyvm", :id, "--graphicscontroller", "vmsvga"]
      end
    end
  end
end