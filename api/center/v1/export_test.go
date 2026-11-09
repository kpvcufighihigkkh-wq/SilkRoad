package v1

// primaryRole 是包私有函数，v1_test 无法直接调用。
// 这里暴露一个只读测试别名：该文件仅参与测试编译，不会进入生产二进制，
// 也不改变任何生产行为。测试落在 v1_test 包，与 lot_test.go / edge_test.go 一致。
var PrimaryRole = primaryRole

// 报表端点的时间与 top 参数解析同样是包私有的，测试无法直接调用。
// 同样只参与测试编译，不进入生产二进制。
var ParseTimeRange = parseTimeRange
var ParseTopN = parseTopN
