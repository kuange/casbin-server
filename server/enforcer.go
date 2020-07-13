// Copyright 2018 The casbin Authors. All Rights Reserved.
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

package server

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"

	pb "github.com/kuange/casbin-server/proto"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// Server is used to implement proto.CasbinServer.
type Server struct {
	enforcerMap map[int]*casbin.Enforcer
	adapterMap  map[int]persist.Adapter
}

func NewServer() *Server {
	s := Server{}

	s.enforcerMap = map[int]*casbin.Enforcer{}
	s.adapterMap = map[int]persist.Adapter{}

	return &s
}

func (s *Server) getEnforcer(handle int) (*casbin.Enforcer, error) {
	if _, ok := s.enforcerMap[handle]; ok {
		return s.enforcerMap[handle], nil
	} else {
		return nil, errors.New("enforcer not found")
	}
}

func (s *Server) getAdapter(handle int) (persist.Adapter, error) {
	if _, ok := s.adapterMap[handle]; ok {
		return s.adapterMap[handle], nil
	} else {
		return nil, errors.New("adapter not found")
	}
}

func (s *Server) addEnforcer(e *casbin.Enforcer) int {
	cnt := len(s.enforcerMap)
	if cnt == 0 {
		s.enforcerMap = map[int]*casbin.Enforcer{}
	}
	s.enforcerMap[cnt] = e
	return cnt
}

func (s *Server) addAdapter(a persist.Adapter) int {
	cnt := len(s.adapterMap)
	if cnt == 0 {
		s.adapterMap = map[int]persist.Adapter{}
	}
	s.adapterMap[cnt] = a
	return cnt
}

func (s *Server) NewEnforcer(ctx context.Context, in *pb.NewEnforcerRequest) (*pb.NewEnforcerReply, error) {
	var a persist.Adapter
	var e *casbin.Enforcer

	if in.AdapterHandle != -1 {
		var err error
		a, err = s.getAdapter(int(in.AdapterHandle))
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}
	}

	if in.ModelText == "" {
		cfg := LoadConfiguration("config/connection_config.json")
		data, err := ioutil.ReadFile(cfg.Enforcer)
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}
		in.ModelText = string(data)
	}

	if a == nil {
		m, err := model.NewModelFromString(in.ModelText)
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}

		e, err = casbin.NewEnforcer(m)
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}
	} else {
		m, err := model.NewModelFromString(in.ModelText)
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}

		e, err = casbin.NewEnforcer(m, a)
		if err != nil {
			return &pb.NewEnforcerReply{Handler: 0}, err
		}
	}
	h := s.addEnforcer(e)

	return &pb.NewEnforcerReply{Handler: int32(h)}, nil
}

func (s *Server) NewAdapter(ctx context.Context, in *pb.NewAdapterRequest) (*pb.NewAdapterReply, error) {
	a, err := newAdapter(in)
	if err != nil {
		return nil, err
	}

	h := s.addAdapter(a)

	return &pb.NewAdapterReply{Handler: int32(h)}, nil
}

func (s *Server) parseParam(param, matcher string) (interface{}, string) {
	if strings.HasPrefix(param, "ABAC::") {
		attrList, err := resolveABAC(param)
		if err != nil {
			panic(err)
		}
		for k, v := range attrList.nameMap {
			old := "." + k
			if strings.Contains(matcher, old) {
				matcher = strings.Replace(matcher, old, "."+v, -1)
			}
		}
		return attrList, matcher
	} else {
		return param, matcher
	}
}

func (s *Server) Enforce(ctx context.Context, in *pb.EnforceRequest) (*pb.BoolReply, error) {
	e, err := s.getEnforcer(int(in.EnforcerHandler))
	if err != nil {
		return &pb.BoolReply{Res: false}, err
	}
	var param interface{}
	params := make([]interface{}, 0, len(in.Params))
	m := e.GetModel()["m"]["m"].Value

	for index := range in.Params {
		param, m = s.parseParam(in.Params[index], m)
		params = append(params, param)
	}

	res, err := e.EnforceWithMatcher(m, params...)
	if err != nil {
		return &pb.BoolReply{Res: false}, err
	}

	return &pb.BoolReply{Res: res}, nil
}

func (s *Server) LoadPolicy(ctx context.Context, in *pb.EmptyRequest) (*pb.EmptyReply, error) {
	e, err := s.getEnforcer(int(in.Handler))
	if err != nil {
		return &pb.EmptyReply{}, err
	}

	err = e.LoadPolicy()

	return &pb.EmptyReply{}, err
}

func (s *Server) SavePolicy(ctx context.Context, in *pb.EmptyRequest) (*pb.EmptyReply, error) {
	e, err := s.getEnforcer(int(in.Handler))
	if err != nil {
		return &pb.EmptyReply{}, err
	}

	err = e.SavePolicy()

	return &pb.EmptyReply{}, err
}
