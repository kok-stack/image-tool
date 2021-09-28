# store

```shell
main pull --image ccr.ccs.tencentyun.com/k8s-test/test:etcd-v1 --savePath ./test/image
main untar --imagePath test/image/ccr.ccs.tencentyun.com-k8s-test-test-etcd-v1.tar.gz --tempPath ./test/tmp/ --distPath ./test/dist/ 
```