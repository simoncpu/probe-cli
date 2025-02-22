package main

//
// Building C dependencies: OpenSSL
//
// Adapted from https://github.com/guardianproject/tor-android
// SPDX-License-Identifier: BSD-3-Clause
//

import (
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/apex/log"
	"github.com/ooni/probe-cli/v3/internal/cmd/buildtool/internal/buildtoolmodel"
	"github.com/ooni/probe-cli/v3/internal/must"
	"github.com/ooni/probe-cli/v3/internal/runtimex"
	"github.com/ooni/probe-cli/v3/internal/shellx"
)

// cdepsOpenSSLBuildMain is the script that builds OpenSSL.
func cdepsOpenSSLBuildMain(globalEnv *cBuildEnv, deps buildtoolmodel.Dependencies) {
	topdir := deps.AbsoluteCurDir() // must be mockable
	work := cdepsMustMkdirTemp()
	restore := cdepsMustChdir(work)
	defer restore()

	// See https://github.com/Homebrew/homebrew-core/blob/master/Formula/openssl@1.1.rb
	cdepsMustFetch("https://www.openssl.org/source/openssl-1.1.1t.tar.gz")
	deps.VerifySHA256( // must be mockable
		"8dee9b24bdb1dcbf0c3d1e9b02fb8f6bf22165e807f45adeb7c9677536859d3b",
		"openssl-1.1.1t.tar.gz",
	)
	must.Run(log.Log, "tar", "-xf", "openssl-1.1.1t.tar.gz")
	_ = deps.MustChdir("openssl-1.1.1t") // must be mockable

	mydir := filepath.Join(topdir, "CDEPS", "openssl")
	for _, patch := range cdepsMustListPatches(mydir) {
		must.Run(log.Log, "git", "apply", patch)
	}

	localEnv := &cBuildEnv{
		CFLAGS:   []string{"-Wno-macro-redefined"},
		CXXFLAGS: []string{"-Wno-macro-redefined"},
	}
	mergedEnv := cBuildMerge(globalEnv, localEnv)
	envp := cBuildExportOpenSSL(mergedEnv)

	// QUIRK: OpenSSL-1.1.1t wants ANDROID_NDK_HOME
	if mergedEnv.ANDROID_NDK_ROOT != "" {
		envp.Append("ANDROID_NDK_HOME", mergedEnv.ANDROID_NDK_ROOT)
	}

	// QUIRK: OpenSSL-1.1.1t wants the PATH to contain the
	// directory where the Android compiler lives.
	if mergedEnv.BINPATH != "" {
		envp.Append("PATH", cdepsPrependToPath(mergedEnv.BINPATH))
	}

	argv := runtimex.Try1(shellx.NewArgv(
		"./Configure", "no-comp", "no-dtls", "no-ec2m", "no-psk", "no-srp",
		"no-ssl2", "no-ssl3", "no-camellia", "no-idea", "no-md2", "no-md4",
		"no-mdc2", "no-rc2", "no-rc4", "no-rc5", "no-rmd160", "no-whirlpool",
		"no-dso", "no-hw", "no-ui-console", "no-shared", "no-unit-test",
		globalEnv.OPENSSL_COMPILER,
	))
	if globalEnv.OPENSSL_API_DEFINE != "" {
		argv.Append(globalEnv.OPENSSL_API_DEFINE)
	}
	argv.Append("--libdir=lib", "--prefix=/", "--openssldir=/")
	runtimex.Try0(shellx.RunEx(defaultShellxConfig(), argv, envp))

	// QUIRK: we need to supply the PATH because OpenSSL's configure
	// isn't as cool as the usual GNU configure unfortunately.
	runtimex.Try0(shellx.RunEx(
		defaultShellxConfig(),
		runtimex.Try1(shellx.NewArgv(
			"make", "-j", strconv.Itoa(runtime.NumCPU()),
		)),
		envp,
	))

	must.Run(log.Log, "make", "DESTDIR="+globalEnv.DESTDIR, "install_dev")
	must.Run(log.Log, "rm", "-rf", filepath.Join(globalEnv.DESTDIR, "lib", "pkgconfig"))
}
