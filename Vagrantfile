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
        echo "=== [Vagrant Fix] Настройка стабильного DNS для моста ==="
        rm -f /etc/resolv.conf
        echo -e "nameserver 8.8.8.8\nnameserver 1.1.1.1" > /etc/resolv.conf

        if [ -f /etc/debian_version ]; then
          echo "=== [Vagrant Fix] Исправление привязки GRUB к диску ==="
          PRIMARY_DISK=$(lsblk -ndrio NAME,TYPE | awk '$2=="disk" {print "/dev/"$1; exit}')
          
          echo "grub-pc grub-pc/install_devices string $PRIMARY_DISK" | debconf-set-selections
        fi
      SHELL
      
      node.vm.provider "virtualbox" do |vb|
        vb.memory = "8192"
        vb.cpus = 4
        vb.name = "servehub-test-#{name}"
        vb.customize ["modifyvm", :id, "--graphicscontroller", "vmsvga"]
      end
    end
  end
end