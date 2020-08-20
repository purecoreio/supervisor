# machine supervisor

Manage your instances from the [purecore.io](https://purecore.io) pannel using this supervisor. You need a **premium+ account** in order to use this feature. **This tool only runs on Linux and MacOS**

Please, use MacOS for development purposes only, no user control for chroot and user creating for sftp access is available when using MacOS)!

# networking considerations
Please, keep in mind this tool will use the port 31518 for the socket server and ports ranging from 31520 to 33000 for the hosted containers

# installation using brew (MacOS)
install docker from [here](https://hub.docker.com/editions/community/docker-ce-desktop-mac/)
```console
brew install git
brew install node
git clone https://github.com/purecoreio/machine-supervisor.git
cd machine-supervisor
npm install
sudo npm start
```

# installation for debian-based Linux
Such as ubuntu...
```console
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo apt install git-all
curl -sL https://deb.nodesource.com/setup_14.x | sudo -E bash -
sudo apt-get install -y nodejs
git clone https://github.com/purecoreio/machine-supervisor.git
cd machine-supervisor
npm install
sudo npm start
```

# installation for fedora-based Linux
Such as CentOS...
```console
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo dnf install git-all
curl -sL https://rpm.nodesource.com/setup_14.x | sudo bash -
git clone https://github.com/purecoreio/machine-supervisor.git
cd machine-supervisor
npm install
sudo npm start
```

# development
make sure you installed all the dependencies using the previous instructions, then; realize changes to /src/ or index.js and run
```console
sudo npm install -g typescript
npm run build
```
test with
```console
sudo npm start
```
