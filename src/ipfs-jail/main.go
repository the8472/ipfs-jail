package main;

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"os/signal"
	"syscall"
	"strings"
	"strconv"
	"path/filepath"
	"time"
	"log"
)

var run = exec.Command


func main() {
	
	user_name := "ipfs"
	
	ipfs_user, err := user.Lookup(user_name)
	
	if err != nil { log.Fatalln(err) }
	
	jail_name := "ipfs-jail"
	
	
	dev := "br0"
	vdev := "eth0"
	binary, err := exec.LookPath("ipfs")
	if err != nil { log.Fatalln(err) }
	
	
	user_home := ipfs_user.HomeDir
	
	//fmt.Println(user_home)
	
	repo := user_home + "/.ipfs/"
	real_repo, err := filepath.EvalSymlinks(repo)
	if err != nil { log.Fatalln(err) }
	
	uid, err := strconv.Atoi(ipfs_user.Uid)
	if err != nil { log.Panicln(err) }
	gid, err := strconv.Atoi(ipfs_user.Gid)
	if err != nil { log.Panicln(err) }
	
	daemon_cmd := make(chan *exec.Cmd)
	
	go func() {
		//fmt.Println(uid, gid)
		
		cmd := run("firejail", "--profile=/etc/ipfs/jail/ipfs-daemon.profile", "--name="+jail_name, "--net="+dev, "--ip=none", "--private="+real_repo, "--env=IPFS_PATH="+user_home, "bash", "-c" , "sleep inf ; "+binary+" daemon")
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
    	cmd.Stdout = os.Stdout
    	cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil { log.Panicln(err) }
		
		// need to wait for ail to spawn its sub-processes
		// TODO: loop until PID is available
		time.Sleep(time.Duration(3*time.Second))
		
		daemon_cmd <- cmd 
	}()
	
	daemon := <- daemon_cmd
	pid := daemon.Process.Pid
	
	/*
	func(){
		cmd := run("ps", "faux")
    	cmd.Stdout = os.Stdout
    	cmd.Stderr = os.Stderr
    	cmd.Run()		
	}()*/
	
	fmt.Println("pid: ", pid)
	
	children, _ := run("pgrep", "-P", strconv.Itoa(pid)).Output()
	fmt.Println("children: ", children)
	child := strings.Split(string(children), "\n")[0]
	
	fmt.Println("child: ", child)
	
	filters := func(name string) {
		cmd := run("nsenter", "-n", "-t", child, name)
    	cmd.Stdout = os.Stdout
    	cmd.Stderr = os.Stderr
		rules, _ := os.Open("/etc/ipfs/jail/restricted-local.net")
		defer rules.Close()
		cmd.Stdin = rules
		cmd.Run()
	}
	
	filters("ip6tables-restore")
	filters("iptables-restore")

	addr := run("nsenter", "-n", "-t", child, "ip", "addr", "show")
   	addr.Stdout = os.Stdout
   	addr.Stderr = os.Stderr
	addr.Run()

	dhcp := run("nsenter", "-u", "-i", "-p", "-m", "-n", "--target", child, "dhclient", "-v", vdev, "-pf", "/tmp/dh4.pid", "-lf", "/tmp/dh4.lease")
   	dhcp.Stdout = os.Stdout
   	dhcp.Stderr = os.Stderr
	dhcp.Run()
	
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
}
