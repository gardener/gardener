// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os/exec"
	"path/filepath"

	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/gardener/gardener/pkg/localprovider"
)

var (
	port         = flag.String("port", ":3777", "The server port")
	vagrantDir   = flag.String("vagrant-dir", "vagrant", "The directory containing the Vagrantfile")
	userdataPath = flag.String("userdata-path", "dev/user-data", "The path in which the user-data file will be created")
)

// server holds the absolute path of the vagrant directory
type server struct {
	vagrantDir   string
	userdataPath string
}

// Start creates a vagrant machine from a user-data
func (s *server) Start(ctx context.Context, in *pb.StartRequest) (*pb.StartReply, error) {
	fmt.Println("Got start request. Creating machine...")
	err := ioutil.WriteFile(s.userdataPath, []byte(in.Cloudconfig), 0644)
	if err != nil {
		fmt.Printf("Error writing config %v", err)
		return nil, status.Errorf(codes.Internal, "Error writing config: %v", err)
	}
	message, err := s.runCommand("up")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error starting machine: %v", err)
	}
	fmt.Println("\nMachine created successfully.")
	return &pb.StartReply{Message: message}, nil
}

// Start deletes a vagrant machine
func (s *server) Delete(ctx context.Context, in *pb.DeleteRequest) (*pb.DeleteReply, error) {
	fmt.Println("Got delete request. Deleting machine...")
	message, err := s.runCommand("destroy", "-f")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error deleting machine: %v", err)
	}
	fmt.Println("\nMachine deleted successfully.")
	return &pb.DeleteReply{Message: message}, nil
}

func main() {
	flag.Parse()
	absVagrantDir, err := filepath.Abs(*vagrantDir)
	if err != nil {
		log.Fatalf("failed to get the vagrant directory: %v", err)
	}
	userdataAbsPath, err := filepath.Abs(*userdataPath)
	if err != nil {
		log.Fatalf("failed to get the user-data path: %v", err)
	}
	lis, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Listening on %s", *port)
	log.Printf("Vagrant directory %s", absVagrantDir)
	log.Printf("user-data path %s", userdataAbsPath)

	s := grpc.NewServer()
	pb.RegisterLocalServer(s, &server{
		vagrantDir:   absVagrantDir,
		userdataPath: userdataAbsPath,
	})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s *server) runCommand(arguments ...string) (string, error) {
	cmd := exec.Command("vagrant", arguments...)
	cmd.Dir = s.vagrantDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer

	// Combine STDOUT and STDERR in a single stream and then duplicate it
	// in order to to print it while outputting and then at the end.
	reader := io.TeeReader(io.MultiReader(stdout, stderr), &buffer)
	scanner := bufio.NewScanner(reader)

	go func() {
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		return "", err
	}

	waitErr := cmd.Wait()

	output, err := ioutil.ReadAll(&buffer)
	if err != nil {
		return "", err
	}

	return string(output), waitErr
}
