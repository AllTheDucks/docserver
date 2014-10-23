CGO_ENABLED=0
GOOS=linux
/usr/local/go/bin/go build -a -tags netgo --ldflags '-w' .
sudo docker build -t "registry.alltheducks.com:5000/docserver" .

