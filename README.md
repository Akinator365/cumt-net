
[cumt-net cumt校园网自动登录 ](https://github.com/Akinator365/cumt-net)
======================

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