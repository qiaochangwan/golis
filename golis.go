package golis

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
)

//系统变量定义

var (
	GolisHandler IoHandler                //事件处理
	Unpacket     func([]byte) interface{} //拆包
	Packet       func(interface{}) []byte //封包
)

//定义session
type Iosession struct {
	SesionId   int      //session唯一表示
	Connection net.Conn //连接
}

//session写入数据
func (this *Iosession) Write(message interface{}) {
	//触发消息发送事件
	GolisHandler.MessageSent(this, message)
	data := Packet(message)
	totalLen := len(data)
	this.Connection.Write(append(IntToBytes(totalLen), data...))
}

//关闭连接
func (this *Iosession) Close() {
	GolisHandler.SessionClosed(this)
	this.Connection.Close()
}

//事件触发接口定义
type IoHandler interface {
	//session打开
	SessionOpened(session *Iosession)
	//session关闭
	SessionClosed(session *Iosession)
	//收到消息时触发
	MessageReceived(session *Iosession, message interface{})
	//消息发送时触发
	MessageSent(session *Iosession, message interface{})
}

//服务器端运行golis
//netPro：运行协议参数，tcp/udp
//laddr ：程序监听ip和端口，如127.0.0.1:8080
func Run(netPro, laddr string) {
	Log("初始化系统完成")
	netLis, err := net.Listen(netPro, laddr)
	CheckError(err)
	defer netLis.Close()
	Log("等待客户端连接...")
	for {
		conn, err := netLis.Accept()
		if err != nil {
			continue
		}
		go connectHandle(conn)
	}
}

//客户端程序连接服务器
func Dial(netPro, laddr string) {
	conn, err := net.Dial(netPro, laddr)
	CheckError(err)
	go connectHandle(conn)
}

//处理新连接
func connectHandle(conn net.Conn) {
	defer conn.Close()
	//声明一个临时缓冲区，用来存储被截断的数据
	tmpBuffer := make([]byte, 0)
	buffer := make([]byte, 1024)

	//声明一个管道用于接收解包的数据
	readerChannel := make(chan []byte, 16)
	//创建session
	session := Iosession{Connection: conn}
	//触发sessionCreated事件
	GolisHandler.SessionOpened(&session)

	exitChan := make(chan bool, 0)
	go reader(&session, readerChannel, exitChan)

	flag := true
	for flag {

		n, err := conn.Read(buffer)
		//Log(time.Now().Unix(), "tmpBuffer.len:", len(tmpBuffer), "tmpBuffer.cap:", cap(tmpBuffer), "n:", n)

		if err == nil {
			tmpBuffer = unpack(append(tmpBuffer, buffer[:n]...), readerChannel)
		} else {
			//			Log("client is disconnected")
			//session关闭
			GolisHandler.SessionClosed(&session)
			flag = false
			exitChan <- true
			break
		}
	}

}

const (
	constDataLength = 4
)

//解包
func unpack(buffer []byte, readerChannel chan []byte) []byte {
	length := len(buffer)

	var i int
	for i = 0; i < length; i = i + 1 {
		if length < i+constDataLength {
			break
		}
		messageLength := BytesToInt(buffer[i : i+constDataLength])
		if length < i+constDataLength+messageLength {
			break
		}
		data := make([]byte, messageLength)
		copy(data, buffer[i+constDataLength:i+constDataLength+messageLength])
		//		data := buffer[i+constDataLength : i+constDataLength+messageLength]
		readerChannel <- data
		i += constDataLength + messageLength - 1
	}

	if i == length {
		buffer = nil
		return make([]byte, 0)
	}
	return buffer[i:]
}

//协议中查看协议头是否满足一个协议报
func getReadyData(buffer []byte) ([]byte, []byte, error) {
	length := len(buffer)
	//	Log("length = ", length)
	if length >= 4 {
		totalLen := BytesToInt(buffer[0:4]) //get totalLen
		if totalLen == 0 {
			return make([]byte, 0), nil, errors.New("msg is null")
		} else if totalLen <= length-4 {
			return buffer[totalLen+4:], buffer[4:totalLen], nil
		}

	}
	return buffer, nil, errors.New("msg is not ready")
}

func reader(session *Iosession, readerChannel chan []byte, exitChan chan bool) {
	for {
		select {
		case data := <-readerChannel:
			readFromData(session, data)
		case <-exitChan:
			break
		}
	}
}

//从准备好的数据读取
func readFromData(session *Iosession, data []byte) {
	message := Unpacket(data) //拆包
	//收到消息时到达
	GolisHandler.MessageReceived(session, message)
}

//整形转换成字节
func IntToBytes(n int) []byte {
	x := int32(n)

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

//字节转换成整形
func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)

	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	return int(x)
}

//整形uint32转换成字节
func Uint32ToBytes(n uint32) []byte {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, n)
	return bytesBuffer.Bytes()
}

//字节转换成uint32
func BytesToUint32(b []byte) uint32 {
	bytesBuffer := bytes.NewBuffer(b)
	var x uint32
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return x
}

//整形32转换成字节数据
func Int32ToBytes(n int32) []byte {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, n)
	return bytesBuffer.Bytes()
}

//字节转换成整形int32
func BytesToint32(b []byte) int32 {
	bytesBuffer := bytes.NewBuffer(b)
	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return x
}

//整形64转换成字节
func Int64ToBytes(n int64) []byte {
	x := int64(n)

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

//字节转换成整形64
func BytesToInt64(b []byte) int64 {
	bytesBuffer := bytes.NewBuffer(b)

	var x int64
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	return int64(x)
}

//整形64转换成字节
func Int8ToBytes(n int8) []byte {
	x := int8(n)

	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

//字节转换成整形8
func BytesToInt8(b []byte) int8 {
	bytesBuffer := bytes.NewBuffer(b)

	var x int8
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	return int8(x)
}

//简单日志输出
func Log(v ...interface{}) {
	fmt.Println(v...)
}

//检查错误并退出程序
func CheckError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fatal error:%s", err.Error())
		os.Exit(1)
	}
}
