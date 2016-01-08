package main

import (
	"flag"
	"fmt"
	_ "log"
	"os"
	"os/exec"
	_ "os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	_ "strings"
	"syscall"
	_ "time"
)

var run = exec.Command

type Conf struct {
	User       *user.User
	JailName   string
	HostDev    string
	JailDev    string
	Executable string
	RepoDir    string
}

func (conf *Conf) populate() {
	envPath := os.Getenv("IPFS_PATH")

	if stat, err := os.Stat(envPath); conf.RepoDir == "" && err == nil && stat.IsDir() {
		fmt.Println("from env")
		conf.RepoDir = envPath

		// infer user from repo owner
		uid := stat.Sys().(*syscall.Stat_t).Uid
		user, err := user.LookupId(fmt.Sprint(uid))
		if uid != 0 && err == nil && conf.User == nil && user != nil {
			fmt.Println("infered user " + user.Username)
			conf.User = user
		}
	}

	// default user: ipfs
	if user, err := user.Lookup("ipfs"); conf.User == nil && err != nil {
		fmt.Println("using default user")
		conf.User = user
	}

	// default repo ~/.ipfs
	if conf.User != nil && conf.RepoDir == "" {
		fmt.Println("default repo")
		repo := filepath.Join(conf.User.HomeDir, ".ipfs")
		if stat, err := os.Stat(repo); err != nil && stat.IsDir() {
			conf.RepoDir = repo
		}
	}

	if binary, err := exec.LookPath("ipfs"); conf.Executable == "" && err != nil {
		fmt.Println("binary from PATH")
		conf.Executable = binary
	}

	conf.JailName = "ipfs-jail"
	conf.HostDev = "br0"
	conf.JailDev = "eth0"
}

var nesting string
var conf = new(Conf)
var dhcp *exec.Cmd

func processArgs() {
	flag.StringVar(&nesting, "nesting", "host", "host/configure-jail/run-jailed")
	userName := flag.String("user", "", "user name")
	repo := flag.String("repo", "", "ipfs repo path")
	flag.Parse()
	
	fmt.Println("user", *userName)
	fmt.Println("repo", *repo)
	fmt.Println("nesting", nesting)
	fmt.Println("rest", flag.Args())

	if(userName != nil && *userName != "") {
		user, err := user.Lookup(*userName)
		if(err != nil) {
			panic("could not lookup user by name " + *userName)
		}
		
		conf.User = user
	}

	if *repo != "" {
		conf.RepoDir = *repo
	}

}

func selfPath() string {
	self, err := filepath.EvalSymlinks("/proc/self/exe")
	if err != nil {
		panic("could not find self ("+os.Args[0]+")")
	}
	return self
}

func startSandbox() {
	// "--profile=/etc/ipfs/jail/ipfs-daemon.profile", "--name="+jail_name, "--net="+dev, "--ip=none", "--private="+real_repo, "--env=IPFS_PATH="+user_home, "bash", "-c", "sleep inf ; "+binary+" daemon"
	cmd := run("firejail", "--noprofile", "--net="+conf.HostDev, "--ip=none", "--name="+conf.JailName+"-outer", "--shell=bash",  selfPath(), "--nesting=configure-jail", "--user="+conf.User.Username, "--repo="+conf.RepoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func configureSandbox() {
	filters := func(name string) {
		cmd := run(name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		rules, _ := os.Open("/etc/ipfs/jail/restricted-local.net")
		defer rules.Close()
		cmd.Stdin = rules
		cmd.Run()
	}

	filters("ip6tables-restore")
	filters("iptables-restore")
	
	dhcp = run("dhclient", "-v", conf.JailDev, "-pf", "/tmp/dh4.pid", "-lf", "/tmp/dh4.lease")
	dhcp.Stdout = os.Stdout
	dhcp.Stderr = os.Stderr
	dhcp.Start()
}

func cleanupSandbox() {
	dhcp.Process.Kill()
	dhcp.Wait()
}

func startInnerSandbox()  {
	uid, _ := strconv.Atoi(conf.User.Uid)
	gid, _ := strconv.Atoi(conf.User.Gid)
	cmd := run("firejail", "--force", "--profile=/etc/ipfs/jail/ipfs-daemon.profile", "--name="+conf.JailName, "--shell=bash", selfPath(), "--nesting=run-jailed", "--user="+conf.User.Username, "--repo="+conf.RepoDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func startTargetProcess() {
	fmt.Println("executable", conf.Executable)
	cmd := run(conf.Executable, "daemon")
	cmd.Env = []string{"IPFS_PATH="+conf.RepoDir}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func main() {
	processArgs()
	conf.populate()
	
	if(conf.User == nil) {
		panic("no user")
	}
	if(conf.RepoDir == "") {
		panic("no repo")
	}
	if(conf.Executable == "") {
		panic("no executable found")
	}
	
	switch nesting {
	case "host":
		startSandbox()
	case "configure-jail":
		configureSandbox()
		startInnerSandbox()
		cleanupSandbox()
	case "run-jailed":
		startTargetProcess()
	}
	
	/*

	cleanup := func() {
		fmt.Println("cleanup")
		dhcp.Process.Signal(syscall.SIGKILL)
	}

	defer cleanup()

	run("firejail", "--join="+jail_name, "pkill", "sleep").Run()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("got signal")
		cleanup()
	}()

	daemon.Wait()
	
	*/
}
