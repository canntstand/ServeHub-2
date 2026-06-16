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
      
      node.vm.provision "shell", inline: <<-SHELL
        if [ -f /etc/arch-release ]; then
          echo "Подготовка Arch..."
          pacman -Sy --noconfirm archlinux-keyring
          pacman -Syu --noconfirm
        elif [ -f /etc/debian_version ]; then
          echo "Обновление Debian/Ubuntu и установка свежего ядра..."
          apt-get update
          
          apt-get install -y linux-image-generic linux-headers-generic
          
          touch /var/run/reboot-required
        fi
      SHELL

      node.vm.provision "shell", id: "reboot", inline: <<-SHELL
        if [ -f /var/run/reboot-required ]; then
          echo "Ядро обновлено. Перезагрузка ВМ..."
          setsid shutdown -r now &
          exit 0
        fi
      SHELL
      
      node.vm.provider "virtualbox" do |vb|
        vb.memory = "4096"
        vb.cpus = 3
        vb.name = "servehub-test-#{name}"
        vb.customize ["modifyvm", :id, "--graphicscontroller", "vmsvga"]
      end
    end
  end
end