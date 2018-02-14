// Copyright 2018 The Gardener Authors.
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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os/exec"
	"path/filepath"

	"google.golang.org/grpc/reflection"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/gardener/gardener/pkg/vagrantprovider"
)

var (
	port       = flag.String("port", ":3777", "The server port")
	vagrantDir = flag.String("vagrant-dir", "vagrant", "The directory conaining the Vagrantfile")
)

// server holds the absolute path of the vagrant directory
type server struct {
	vagrantDir string
}

// Start creates a vagrant machine from a user-data
func (s *server) Start(ctx context.Context, in *pb.StartRequest) (*pb.StartReply, error) {
	fmt.Println("Got start request. Creating machine")
	cmd := exec.Command("vagrant", "up")
	cmd.Dir = s.vagrantDir
	absPath, _ := filepath.Abs("dev/user-data")
	err := ioutil.WriteFile(absPath, []byte(in.Cloudconfig), 0644)
	if err != nil {
		fmt.Printf("Error writing config %v", err)
		return nil, status.Errorf(codes.Internal, "Error writing config: %v", err)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error starting machine: %s", string(output))
		return nil, status.Errorf(codes.Internal, "Error starting machine: %v", err)
	}
	message := string(output)
	fmt.Println(message)
	return &pb.StartReply{Message: message}, nil
}

// Start deltes a vagrant machine
func (s *server) Delete(ctx context.Context, in *pb.DeleteRequest) (*pb.DeleteReply, error) {
	cmd := exec.Command("vagrant", "destroy", "-f")
	cmd.Dir = s.vagrantDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error stopping machine: %s", string(output))
		return nil, status.Errorf(codes.Internal, "Error stopping machine: %v", err)
	}
	message := string(output)
	fmt.Println(message)
	return &pb.DeleteReply{Message: message}, nil
}

func main() {
	flag.Parse()
	absVagrantDir, err := filepath.Abs(*vagrantDir)
	if err != nil {
		log.Fatalf("failed to get the vagrant directory: %v", err)
	}
	lis, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Listening on %s", *port)
	log.Printf("Vagrant directory %s", absVagrantDir)

	s := grpc.NewServer()
	pb.RegisterVagrantServer(s, &server{
		vagrantDir: absVagrantDir,
	})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
