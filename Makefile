
.PHONY: all clean realclean fmt openbsd

all:
	./build -s

openbsd:
	./build -s --arch=openbsd-amd64

clean realclean:
	-rm -f ./bin

fmt:
	gofmt -w src/*.go

