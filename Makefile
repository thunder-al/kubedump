run:
	cargo run

release:
	cargo build --release
	upx -9 ./target/release/kubedump ./target/release/kubedump.exe || true
	cp ./target/release/kubedump ./kubedump || cp ./target/release/kubedump.exe ./kubedump.exe
