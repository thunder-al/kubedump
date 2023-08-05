build-release:
	rm -rf ./dist
	mkdir ./dist
	case "$$OSTYPE" in \
	  msys|cygwin|win32) \
		go build -ldflags "-s -w" -o ./dist/kubedump.exe; \
		upx ./dist/kubedump.exe || true; \
		;; \
	  *) \
		go build -ldflags "-s -w" -o ./dist/kubedump; \
		;; \
	esac
