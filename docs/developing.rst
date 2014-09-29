Developer Information
=====================

Momonga is still highly under development. for now, this section tells you how to build momonga:

	# Install go
	curl -O https://storage.googleapis.com/golang/go1.3.2.linux-amd64.tar.gz
	sudo tar -C /usr/local -zxf go1.3.2.linux-amd64.tar.gz

	echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
	source ~/.bashrc

	# Building momonga
	mkdir momonga && cd momonga
	export GOPATH=`pwd`
	go get github.com/chobie/momonga/momonga
	go build -o momonga github.com/chobie/momonga/momonga


.. toctree::
   :maxdepth: 1

