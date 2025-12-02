.PHONY: generate clean

generate:
	@echo "Generating Go code from Protocol Buffers..."
	@mkdir -p ../pb
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		control.proto

clean:
	rm -f *.pb.go

