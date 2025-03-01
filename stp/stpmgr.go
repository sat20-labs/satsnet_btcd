package stp

import (
	"fmt"
	"log"
	"plugin"
)

var _stpmar *plugin.Plugin

func LoadSTP() error {
	if _stpmar != nil {
		return nil
	}

	// 打开插件文件
	p, err := plugin.Open("./stpd.so")
	if err != nil {
		log.Printf("plugin.Open failed. %v", err)
		return err
	}

	symbol, err := p.Lookup("InitSTP")
	if err != nil {
		log.Printf("Lookup InitSTP failed: %v", err)
		return err
	}

	initSTP, ok := symbol.(func() error)
	if !ok {
		log.Printf("symbol type assertion failed")
		return err
	}

	err = initSTP()
	if err != nil {
		log.Printf("initSTP failed: %v", err)
		return err
	}

	_stpmar = p
	return nil
}

func ReleaseSTP() {
	if _stpmar == nil {
		return
	}

	symbol, err := _stpmar.Lookup("ReleaseSTP")
	if err != nil {
		log.Printf("Lookup ReleaseSTP failed: %v", err)
		return 
	}

	releaseSTP, ok := symbol.(func())
	if !ok {
		log.Printf("symbol type assertion failed")
		return 
	}

	releaseSTP()

	_stpmar = nil
}


func SignMsg(msg []byte) ([]byte, error) {
	if _stpmar == nil {
		return nil, fmt.Errorf("STPManager not init")
	}

	symbol, err := _stpmar.Lookup("SignMsg")
	if err != nil {
		log.Printf("Lookup SignMsg failed: %v", err)
		return  nil, err
	}

	signMsg, ok := symbol.(func([]byte) ([]byte, error))
	if !ok {
		log.Printf("symbol type assertion failed")
		return nil, fmt.Errorf("symbol type assertion failed")
	}

	return signMsg(msg)
}
