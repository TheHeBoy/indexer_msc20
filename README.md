# open-indexer
Open source indexer
## Model Sava Order
数据的存储顺序，上一层进行符合过滤条件才会继续到下一层,

- 第一层 transaction and evmlog
- 第二层 inscription 需要符合 eths 的规则，如`data:text/plain;rule=esip6,`
- 第三层 msc20 需要符合 msc20 格式，如 `{"p": "msc-20","op": "deploy","tick": "avav","max": "1463636349000000","lim": "69696969"}`
- 第四层 token 已经部署的 msc20 token
- 第五层 holder token 持有情况
