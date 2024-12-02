
[cumt-net cumt校园网自动登录 ](https://github.com/Akinator365/cumt-net)
======================

### 使用方法:
需要搭配openwrt使用，在路由器系统安装ipk包后，配置登录规则以及代理规则

### 主界面：
![image](https://github.com/Akinator365/luci-app-cumt-net/blob/master/demo-png/main.png)
### 登录规则配置：
![image](https://github.com/Akinator365/luci-app-cumt-net/blob/master/demo-png/login.png)
### Passwall规则配置：
![image](https://github.com/Akinator365/luci-app-cumt-net/blob/master/demo-png/passwall.png)
### 下载源码方法:

 ```Brach
 
    # 下载源码
	
    git clone https://github.com/Akinator365/cumt-net package/cumt-net
    make menuconfig
	
 ``` 
### 配置菜单

 ```Brach
    make menuconfig
	# 找到 Network -> Web Servers/Proxies, 选择 cumt-net, 保存后退出。
 ``` 
 
### 编译

 ```Brach 
    # 编译固件
    make package/cumt-net/{clean,compile} V=s
 ```