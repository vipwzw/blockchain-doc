# LayerZero + Conflux 跨链完整部署方案

---

## 📋 目录

- [项目概述](#项目概述)
- [1. 架构概述](#1-架构概述)
- [2. 智能合约部署](#2-智能合约部署)
- [3. DVN 节点部署](#3-dvn-节点部署)
- [4. 区块链节点部署](#4-区块链节点部署)
- [5. HSM 多云部署](#5-hsm-多云部署)
- [6. 网络架构](#6-网络架构)
- [7. 监控告警](#7-监控告警)
- [8. 运维手册](#8-运维手册)

---

## 项目概述

### 目标

构建一个安全、去中心化的 LayerZero DVN（Decentralized Verifier Network），支持以太坊生态与 Conflux 之间的跨链资产转移。

### 核心特性

| 特性 | 描述 |
|------|------|
| ✅ 去中心化验证 | 自建 DVN，不依赖第三方 |
| ✅ 多云 HSM | AWS + 阿里云 + Google Cloud 分布式密钥 |
| ✅ 自建节点 | 防止 RPC 作弊，完全可信数据源 |
| ✅ 高可用架构 | 多区域部署，无单点故障 |
| ✅ 完全私有网络 | 无公网暴露，最高安全级别 |

### 整体架构预览

```mermaid
flowchart TB
    subgraph Users["用户层"]
        DApp[Web DApp]
        Mobile[Mobile App]
        SDK[SDK/API]
    end

    subgraph SourceChain["源链 (以太坊/Arbitrum)"]
        OFTAdapter[OFTAdapter<br/>锁定 USDT]
        SendLib[SendLib ULN302]
        SrcEndpoint[Endpoint V2]
    end

    subgraph TargetChain["目标链 (Conflux eSpace)"]
        OFT[OFT<br/>铸造 USDT0]
        ReceiveLib[ReceiveLib ULN302]
        DstEndpoint[Endpoint V2]
        DVNContract[DVN 合约]
    end

    subgraph DVNCluster["DVN 集群 (多云部署)"]
        subgraph AWS["AWS 美东"]
            DVN1[DVN Node #1]
            HSM1[CloudHSM]
            ETH1[ETH Node]
        end
        subgraph Aliyun["阿里云 杭州"]
            DVN2[DVN Node #2]
            HSM2[密钥管理]
            ETH2[ETH Node]
        end
        subgraph GCP["Google 东京"]
            DVN3[DVN Node #3]
            HSM3[Cloud KMS]
            ETH3[ETH Node]
        end
    end

    Users --> OFTAdapter
    OFTAdapter --> SrcEndpoint
    SrcEndpoint --> SendLib
    SendLib -.->|跨链消息| DVNCluster
    DVNCluster -.->|验证签名| DVNContract
    DVNContract --> ReceiveLib
    ReceiveLib --> DstEndpoint
    DstEndpoint --> OFT

    DVN1 <--> HSM1
    DVN1 <--> ETH1
    DVN2 <--> HSM2
    DVN2 <--> ETH2
    DVN3 <--> HSM3
    DVN3 <--> ETH3
```

### 技术栈

| 层级 | 技术选型 |
|------|---------|
| **智能合约** | Solidity, Hardhat, LayerZero OFT V2 |
| **DVN 服务** | Go / Rust, gRPC, Redis |
| **区块链节点** | Geth (以太坊), Conflux-Rust |
| **HSM** | AWS CloudHSM, 阿里云密钥管理, Google Cloud KMS |
| **网络** | VPC, VPN/专线, NAT Gateway |
| **容器化** | Docker, Kubernetes |
| **监控** | Prometheus, Grafana, AlertManager |
| **日志** | ELK Stack / CloudWatch |

### 月度成本预估

| 组件 | AWS | 阿里云 | Google Cloud | 总计 |
|------|----:|-------:|-------------:|-----:|
| **DVN 服务器** | $200 | $180 | $200 | $580 |
| **以太坊节点** | $750 | $700 | $750 | $2,200 |
| **Conflux 节点** | $200 | $180 | $200 | $580 |
| **HSM** | $1,500 | $1,000 | $400 | $2,900 |
| **网络/带宽** | $200 | $150 | $200 | $550 |
| **跨云专线** | $300 | $300 | $300 | $900 |
| **监控/日志** | $100 | $80 | $100 | $280 |
| **总计** | **$3,250** | **$2,590** | **$2,150** | **$7,990** |

### 快速开始

```bash
# 1. 克隆部署脚本
git clone https://github.com/your-org/layerzero-cfx-dvn.git
cd layerzero-cfx-dvn

# 2. 配置环境变量
cp .env.example .env
vim .env

# 3. 部署基础设施 (Terraform)
cd infrastructure
terraform init
terraform plan
terraform apply

# 4. 部署智能合约
cd ../contracts
npx hardhat deploy --network ethereum
npx hardhat deploy --network conflux

# 5. 启动 DVN 服务
cd ../dvn
docker-compose up -d
```

---

# 1. 架构概述

## 1.1 系统架构图

```mermaid
flowchart TB
    subgraph UserLayer["用户层 (User Layer)"]
        WebDApp[Web DApp<br/>前端界面]
        MobileApp[Mobile App<br/>移动应用]
        SDKAPI[SDK/API<br/>开发者集成]
        CLI[CLI Tool<br/>命令行]
    end

    subgraph ContractLayer["智能合约层 (Contract Layer)"]
        subgraph SourceChain["源链 (以太坊/Arbitrum/Optimism)"]
            OFTAdapter[OFTAdapter<br/>• lock/unlock<br/>• 持有原生 USDT]
            SendLib[SendLib ULN302<br/>• 消息打包<br/>• 费用计算<br/>• DVN 配置]
            SrcEndpoint[Endpoint V2<br/>• 消息路由<br/>• nonce 管理]
        end

        subgraph TargetChain["目标链 (Conflux eSpace)"]
            OFT[OFT<br/>• mint/burn<br/>• 管理 USDT0]
            ReceiveLib[ReceiveLib ULN302<br/>• 验证签名<br/>• 确认验证状态<br/>• 触发接收]
            DstEndpoint[Endpoint V2<br/>• 消息接收<br/>• 调用 lzReceive]
            DVNContract[DVN 合约<br/>• 接收签名<br/>• 验证 quorum]
        end
    end

    subgraph VerifyLayer["验证层 (Verification Layer)"]
        subgraph DVNCluster["DVN 集群 (分布式验证网络)"]
            subgraph AWSRegion["AWS 美东区域"]
                DVN1[DVN Node #1<br/>Event Listener<br/>Verifier<br/>Signer HSM]
            end
            subgraph AliyunRegion["阿里云 杭州区域"]
                DVN2[DVN Node #2<br/>Event Listener<br/>Verifier<br/>Signer HSM]
            end
            subgraph GCPRegion["Google 东京区域"]
                DVN3[DVN Node #3<br/>Event Listener<br/>Verifier<br/>Signer HSM]
            end
        end
        SignAgg[签名聚合<br/>阈值签名 2/3]
    end

    subgraph InfraLayer["基础设施层 (Infrastructure Layer)"]
        subgraph BlockchainNodes["区块链全节点集群"]
            Geth[Geth 节点 x3]
            ConfluxNode[Conflux 节点 x3]
            ArbNode[Arbitrum 节点]
            OpNode[Optimism 节点]
        end
        subgraph HSMCluster["HSM 集群"]
            AWSCloudHSM[AWS CloudHSM]
            AliyunKMS[阿里云密钥管理]
            GCPKMS[Google Cloud KMS]
        end
        subgraph Monitoring["监控系统"]
            Prometheus[Prometheus]
            Grafana[Grafana]
            AlertManager[AlertManager]
            ELK[ELK Stack]
        end
    end

    UserLayer --> OFTAdapter
    OFTAdapter --> SrcEndpoint
    SrcEndpoint --> SendLib
    SendLib -.->|PacketSent 事件| DVNCluster
    DVN1 --> SignAgg
    DVN2 --> SignAgg
    DVN3 --> SignAgg
    SignAgg -.->|提交签名| DVNContract
    DVNContract --> ReceiveLib
    ReceiveLib --> DstEndpoint
    DstEndpoint --> OFT
```

---

## 1.2 跨链消息流程

### 1.2.1 完整跨链流程 (以太坊 → Conflux)

```mermaid
sequenceDiagram
    autonumber
    participant User as 用户钱包
    participant USDT as USDT 合约
    participant Adapter as OFTAdapter
    participant Endpoint as Endpoint V2
    participant SendLib as SendLib
    participant DVN1 as DVN #1 (AWS)
    participant DVN2 as DVN #2 (阿里云)
    participant DVN3 as DVN #3 (Google)
    participant DVNContract as DVN 合约
    participant RcvLib as ReceiveLib
    participant DstEnd as 目标 Endpoint
    participant OFT as OFT (USDT0)

    rect rgb(240, 248, 255)
        Note over User,SendLib: Step 1: 用户发起跨链
        User->>USDT: approve(adapter, 100 USDT)
        User->>Adapter: send(dstEid=Conflux, to=用户, amount=100)
        Adapter->>USDT: transferFrom(用户, adapter, 100)
        Note right of Adapter: 锁定 100 USDT
        Adapter->>Endpoint: send(dstEid, message, options)
        Endpoint->>SendLib: send(packet)
        SendLib-->>SendLib: emit PacketSent(nonce, srcEid, dstEid, payload)
    end

    rect rgb(255, 250, 240)
        Note over DVN1,DVN3: Step 2: DVN 监听并验证
        SendLib-->>DVN1: 监听 PacketSent 事件
        SendLib-->>DVN2: 监听 PacketSent 事件
        SendLib-->>DVN3: 监听 PacketSent 事件
        
        Note over DVN1: 等待 12 区块确认
        Note over DVN2: 等待 12 区块确认
        Note over DVN3: 等待 12 区块确认
        
        DVN1->>DVN1: 验证交易 + HSM 签名
        DVN2->>DVN2: 验证交易 + HSM 签名
        DVN3->>DVN3: 验证交易 + HSM 签名
    end

    rect rgb(240, 255, 240)
        Note over DVNContract,OFT: Step 3: 目标链执行
        DVN1->>DVNContract: verifyAndSubmit(header, hash, sigs[])
        Note right of DVNContract: 验证 2/3 签名
        DVNContract->>RcvLib: verify(header, payloadHash, confirmations)
        RcvLib->>RcvLib: 记录验证状态
        
        Note over DstEnd: Executor 调用
        DstEnd->>OFT: lzReceive(origin, guid, message)
        OFT->>OFT: _mint(用户, 100 USDT0)
        Note right of OFT: ✅ 用户收到 100 USDT0
    end
```

### 1.2.2 DVN 验证详细流程

```mermaid
flowchart LR
    subgraph SourceChain["源链"]
        Event[PacketSent 事件]
    end

    subgraph DVNNode["DVN 节点"]
        Listen[事件监听器]
        Wait[等待确认<br/>12 blocks]
        Verify[交易验证]
        Sign[HSM 签名]
    end

    subgraph Aggregator["签名聚合"]
        Collect[收集签名]
        Check{达到 2/3?}
        Submit[提交到目标链]
    end

    subgraph TargetChain["目标链"]
        DVNContract[DVN 合约]
        ReceiveLib[ReceiveLib]
    end

    Event --> Listen
    Listen --> Wait
    Wait --> Verify
    Verify --> Sign
    Sign --> Collect
    Collect --> Check
    Check -->|是| Submit
    Check -->|否| Collect
    Submit --> DVNContract
    DVNContract --> ReceiveLib
```

---

## 1.3 组件职责

| 组件 | 部署位置 | 职责 |
|------|---------|------|
| **OFTAdapter** | 源链 | 锁定/解锁原生代币 |
| **OFT** | 目标链 | 铸造/销毁包装代币 |
| **Endpoint** | 所有链 | 消息路由、nonce 管理 |
| **SendLib** | 源链 | 打包消息、触发事件 |
| **ReceiveLib** | 目标链 | 验证签名、确认消息 |
| **DVN 合约** | 目标链 | 接收并验证 DVN 签名 |
| **DVN 节点** | 链下 | 监听事件、签名验证 |
| **区块链节点** | 链下 | 提供可信数据源 |
| **HSM** | 链下 | 安全存储签名密钥 |

---

## 1.4 安全模型

```mermaid
flowchart TB
    subgraph Threats["威胁"]
        T1[单个 DVN 被攻破]
        T2[RPC 作弊]
        T3[私钥泄露]
        T4[单云服务商故障]
        T5[网络攻击]
        T6[重放攻击]
        T7[内部人员作恶]
    end

    subgraph Protections["防护措施"]
        P1[阈值签名 2/3]
        P2[自建全节点]
        P3[HSM 保护]
        P4[多云部署]
        P5[私有网络 + 无公网 IP]
        P6[nonce 机制]
        P7[多方共管 + 审计日志]
    end

    T1 --> P1
    T2 --> P2
    T3 --> P3
    T4 --> P4
    T5 --> P5
    T6 --> P6
    T7 --> P7
```

### 信任假设

| 假设 | 描述 |
|------|------|
| DVN 诚实性 | 至少 2/3 的 DVN 节点是诚实的 |
| 云服务商隔离 | 各云服务商不会同时被攻破 |
| HSM 安全性 | HSM 硬件是安全的 |
| 数据可信性 | 自建区块链节点数据是可信的 |

---

## 1.5 高可用设计

```mermaid
flowchart TB
    subgraph Region1["AWS 美东 (主)"]
        DVN1[DVN + HSM + 全节点]
    end

    subgraph Region2["阿里云 杭州 (备)"]
        DVN2[DVN + HSM + 全节点]
    end

    subgraph Region3["Google 东京 (备)"]
        DVN3[DVN + HSM + 全节点]
    end

    subgraph SharedState["共享状态"]
        Redis[(Redis Cluster)]
    end

    DVN1 <-->|VPN/专线| DVN2
    DVN2 <-->|VPN/专线| DVN3
    DVN3 <-->|VPN/专线| DVN1

    DVN1 --> Redis
    DVN2 --> Redis
    DVN3 --> Redis

    style Region1 fill:#90EE90
    style Region2 fill:#87CEEB
    style Region3 fill:#FFB6C1
```

### 故障转移策略

| 场景 | 处理方式 |
|------|---------|
| 单个区域故障 | 其他 2 个区域继续运行，满足 2/3 阈值 |
| 单个 DVN 节点故障 | 自动切换到健康节点 |
| HSM 故障 | 使用其他区域 HSM 签名 |
| 区块链节点故障 | 自动切换到备用节点 |

---

## 1.6 数据流架构

```mermaid
flowchart LR
    subgraph DataSources["数据源"]
        ETH[以太坊节点]
        CFX[Conflux 节点]
        ARB[Arbitrum 节点]
    end

    subgraph DVNServices["DVN 服务"]
        Listener[事件监听服务]
        Verifier[验证服务]
        Signer[签名服务]
        Submitter[提交服务]
    end

    subgraph Storage["存储"]
        Redis[(Redis<br/>任务队列)]
        Postgres[(PostgreSQL<br/>审计日志)]
    end

    subgraph HSM["HSM"]
        CloudHSM[密钥存储]
    end

    ETH --> Listener
    CFX --> Listener
    ARB --> Listener
    
    Listener --> Redis
    Redis --> Verifier
    Verifier --> Redis
    Redis --> Signer
    Signer <--> CloudHSM
    Signer --> Redis
    Redis --> Submitter
    Submitter --> CFX
    
    Listener --> Postgres
    Verifier --> Postgres
    Signer --> Postgres
    Submitter --> Postgres
```

---

# 2. 智能合约部署

## 2.1 合约架构

```mermaid
flowchart TB
    subgraph Ethereum["以太坊 (源链)"]
        subgraph Deployed1["已部署 (LayerZero)"]
            EthEndpoint[Endpoint V2]
            EthSendLib[SendLib ULN302]
            EthDVNRegistry[DVN Registry]
        end
        subgraph ToDeploy1["需要部署 (我们)"]
            OFTAdapter[OFTAdapter<br/>锁定 USDT]
        end
    end

    subgraph Conflux["Conflux eSpace (目标链)"]
        subgraph Deployed2["已部署 (LayerZero)"]
            CfxEndpoint[Endpoint V2]
            CfxReceiveLib[ReceiveLib ULN302]
            CfxDVNRegistry[DVN Registry]
        end
        subgraph ToDeploy2["需要部署 (我们)"]
            OFT[OFT USDT0<br/>铸造代币]
            CustomDVN[CustomDVN<br/>验证合约]
        end
    end

    OFTAdapter --> EthEndpoint
    EthEndpoint --> EthSendLib
    
    CustomDVN --> CfxReceiveLib
    CfxReceiveLib --> CfxEndpoint
    CfxEndpoint --> OFT

    style ToDeploy1 fill:#90EE90
    style ToDeploy2 fill:#90EE90
```

---

## 2.2 项目结构

```bash
contracts/
├── src/
│   ├── OFTAdapter.sol          # 源链适配器
│   ├── OFT.sol                 # 目标链代币
│   ├── CustomDVN.sol           # 自定义 DVN 合约
│   └── interfaces/
│       ├── ILayerZeroEndpointV2.sol
│       ├── IReceiveLib.sol
│       └── ISendLib.sol
├── script/
│   ├── DeployEthereum.s.sol    # 以太坊部署脚本
│   ├── DeployConflux.s.sol     # Conflux 部署脚本
│   └── ConfigureOApp.s.sol     # 配置脚本
├── test/
│   └── OFT.t.sol
├── foundry.toml
└── hardhat.config.ts
```

---

## 2.3 OFTAdapter 合约 (源链)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { OFTAdapter } from "@layerzerolabs/oft-evm/contracts/OFTAdapter.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title USDTOFTAdapter
 * @notice 源链 USDT 锁定合约
 * @dev 锁定原生 USDT，通过 LayerZero 发送跨链消息
 */
contract USDTOFTAdapter is OFTAdapter {
    
    // 每日跨链限额
    uint256 public dailyLimit;
    uint256 public dailyTransferred;
    uint256 public lastResetTime;
    
    // 单笔最大/最小限额
    uint256 public minAmount;
    uint256 public maxAmount;
    
    // 暂停状态
    bool public paused;
    
    // 白名单（可选）
    mapping(address => bool) public whitelist;
    bool public whitelistEnabled;
    
    event DailyLimitUpdated(uint256 oldLimit, uint256 newLimit);
    event Paused(address account);
    event Unpaused(address account);
    
    error TransferPaused();
    error ExceedsDailyLimit();
    error AmountTooSmall();
    error AmountTooLarge();
    error NotWhitelisted();
    
    constructor(
        address _token,           // USDT 地址
        address _lzEndpoint,      // LayerZero Endpoint
        address _delegate         // 管理员
    ) OFTAdapter(_token, _lzEndpoint, _delegate) Ownable(_delegate) {
        dailyLimit = 1_000_000 * 1e6;  // 100万 USDT
        minAmount = 10 * 1e6;           // 最小 10 USDT
        maxAmount = 100_000 * 1e6;      // 最大 10万 USDT
        lastResetTime = block.timestamp;
    }
    
    /**
     * @notice 重写 _debit 以添加限额检查
     */
    function _debit(
        address _from,
        uint256 _amountLD,
        uint256 _minAmountLD,
        uint32 _dstEid
    ) internal virtual override returns (uint256 amountSentLD, uint256 amountReceivedLD) {
        // 暂停检查
        if (paused) revert TransferPaused();
        
        // 白名单检查
        if (whitelistEnabled && !whitelist[_from]) revert NotWhitelisted();
        
        // 金额检查
        if (_amountLD < minAmount) revert AmountTooSmall();
        if (_amountLD > maxAmount) revert AmountTooLarge();
        
        // 每日限额检查
        _checkAndUpdateDailyLimit(_amountLD);
        
        // 调用父合约逻辑
        return super._debit(_from, _amountLD, _minAmountLD, _dstEid);
    }
    
    function _checkAndUpdateDailyLimit(uint256 _amount) internal {
        // 重置每日计数
        if (block.timestamp >= lastResetTime + 1 days) {
            dailyTransferred = 0;
            lastResetTime = block.timestamp;
        }
        
        if (dailyTransferred + _amount > dailyLimit) {
            revert ExceedsDailyLimit();
        }
        
        dailyTransferred += _amount;
    }
    
    // ============ 管理函数 ============
    
    function setDailyLimit(uint256 _limit) external onlyOwner {
        emit DailyLimitUpdated(dailyLimit, _limit);
        dailyLimit = _limit;
    }
    
    function setAmountLimits(uint256 _min, uint256 _max) external onlyOwner {
        minAmount = _min;
        maxAmount = _max;
    }
    
    function pause() external onlyOwner {
        paused = true;
        emit Paused(msg.sender);
    }
    
    function unpause() external onlyOwner {
        paused = false;
        emit Unpaused(msg.sender);
    }
    
    function setWhitelist(address _user, bool _status) external onlyOwner {
        whitelist[_user] = _status;
    }
    
    function setWhitelistEnabled(bool _enabled) external onlyOwner {
        whitelistEnabled = _enabled;
    }
    
    /**
     * @notice 紧急提取（多签控制）
     */
    function emergencyWithdraw(
        address _token,
        address _to,
        uint256 _amount
    ) external onlyOwner {
        IERC20(_token).transfer(_to, _amount);
    }
}
```

---

## 2.4 OFT 合约 (目标链)

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { OFT } from "@layerzerolabs/oft-evm/contracts/OFT.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title USDT0
 * @notice Conflux 上的 USDT 包装代币
 */
contract USDT0 is OFT {
    
    uint256 public maxTotalSupply;
    bool public paused;
    
    error MintPaused();
    error ExceedsMaxSupply();
    
    constructor(
        string memory _name,
        string memory _symbol,
        address _lzEndpoint,
        address _delegate
    ) OFT(_name, _symbol, _lzEndpoint, _delegate) Ownable(_delegate) {
        maxTotalSupply = 1_000_000_000 * 1e6;  // 10亿上限
    }
    
    function decimals() public pure override returns (uint8) {
        return 6;
    }
    
    function _credit(
        address _to,
        uint256 _amountLD,
        uint32 _srcEid
    ) internal virtual override returns (uint256 amountReceivedLD) {
        if (paused) revert MintPaused();
        if (totalSupply() + _amountLD > maxTotalSupply) revert ExceedsMaxSupply();
        return super._credit(_to, _amountLD, _srcEid);
    }
    
    function setMaxTotalSupply(uint256 _max) external onlyOwner {
        maxTotalSupply = _max;
    }
    
    function pause() external onlyOwner { paused = true; }
    function unpause() external onlyOwner { paused = false; }
}
```

---

## 2.5 自定义 DVN 合约

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";

interface IReceiveLib {
    function verify(
        bytes calldata _packetHeader,
        bytes32 _payloadHash,
        uint64 _confirmations
    ) external;
}

/**
 * @title CustomDVN
 * @notice 自定义 DVN 验证合约
 */
contract CustomDVN is Ownable {
    using ECDSA for bytes32;
    
    mapping(address => bool) public signers;
    address[] public signerList;
    uint256 public quorum;
    mapping(uint32 => address) public receiveLibs;
    mapping(bytes32 => bool) public processedMessages;
    mapping(uint32 => uint64) public requiredConfirmations;
    
    event SignerAdded(address indexed signer);
    event SignerRemoved(address indexed signer);
    event QuorumUpdated(uint256 oldQuorum, uint256 newQuorum);
    event VerificationSubmitted(bytes32 indexed messageHash, uint32 srcEid, uint64 nonce);
    
    error InvalidSignature();
    error InsufficientSignatures();
    error MessageAlreadyProcessed();
    error InvalidReceiveLib();
    error DuplicateSigner();
    error InvalidQuorum();
    
    constructor(
        address[] memory _initialSigners,
        uint256 _quorum,
        address _owner
    ) Ownable(_owner) {
        require(_quorum <= _initialSigners.length && _quorum > 0, "Invalid quorum");
        
        for (uint256 i = 0; i < _initialSigners.length; i++) {
            signers[_initialSigners[i]] = true;
            signerList.push(_initialSigners[i]);
            emit SignerAdded(_initialSigners[i]);
        }
        quorum = _quorum;
    }
    
    function verifyAndSubmit(
        bytes calldata _packetHeader,
        bytes32 _payloadHash,
        uint64 _confirmations,
        bytes[] calldata _signatures
    ) external {
        bytes32 messageHash = keccak256(abi.encodePacked(
            _packetHeader, _payloadHash, _confirmations
        ));
        
        if (processedMessages[messageHash]) revert MessageAlreadyProcessed();
        _verifySignatures(messageHash, _signatures);
        processedMessages[messageHash] = true;
        
        uint32 srcEid = _parseSrcEid(_packetHeader);
        address receiveLib = receiveLibs[srcEid];
        if (receiveLib == address(0)) revert InvalidReceiveLib();
        
        IReceiveLib(receiveLib).verify(_packetHeader, _payloadHash, _confirmations);
        emit VerificationSubmitted(messageHash, srcEid, _parseNonce(_packetHeader));
    }
    
    function _verifySignatures(bytes32 _messageHash, bytes[] calldata _signatures) internal view {
        if (_signatures.length < quorum) revert InsufficientSignatures();
        
        bytes32 ethSignedHash = _messageHash.toEthSignedMessageHash();
        address lastSigner = address(0);
        
        for (uint256 i = 0; i < _signatures.length; i++) {
            address signer = ethSignedHash.recover(_signatures[i]);
            if (!signers[signer]) revert InvalidSignature();
            if (signer <= lastSigner) revert DuplicateSigner();
            lastSigner = signer;
        }
    }
    
    function _parseSrcEid(bytes calldata _packetHeader) internal pure returns (uint32) {
        return uint32(bytes4(_packetHeader[9:13]));
    }
    
    function _parseNonce(bytes calldata _packetHeader) internal pure returns (uint64) {
        return uint64(bytes8(_packetHeader[1:9]));
    }
    
    // 管理函数
    function addSigner(address _signer) external onlyOwner {
        if (signers[_signer]) revert DuplicateSigner();
        signers[_signer] = true;
        signerList.push(_signer);
        emit SignerAdded(_signer);
    }
    
    function setQuorum(uint256 _quorum) external onlyOwner {
        if (_quorum > signerList.length || _quorum == 0) revert InvalidQuorum();
        emit QuorumUpdated(quorum, _quorum);
        quorum = _quorum;
    }
    
    function setReceiveLib(uint32 _srcEid, address _receiveLib) external onlyOwner {
        receiveLibs[_srcEid] = _receiveLib;
    }
    
    function getSignerCount() external view returns (uint256) { return signerList.length; }
    function getAllSigners() external view returns (address[] memory) { return signerList; }
}
```

---

## 2.6 部署流程

```mermaid
flowchart TD
    subgraph Phase1["阶段 1: 准备"]
        A1[配置环境变量]
        A2[准备部署钱包]
        A3[确认 RPC 端点]
        A4[获取 LayerZero 地址]
    end

    subgraph Phase2["阶段 2: 部署源链"]
        B1[部署 Ethereum OFTAdapter]
        B2[部署 Arbitrum OFTAdapter]
        B3[验证合约 Etherscan]
    end

    subgraph Phase3["阶段 3: 部署目标链"]
        C1[部署 Conflux USDT0]
        C2[部署 CustomDVN]
        C3[验证合约 ConfluxScan]
    end

    subgraph Phase4["阶段 4: 配置"]
        D1[配置 Ethereum Peer]
        D2[配置 Conflux Peer]
        D3[配置 DVN 设置]
        D4[设置 ReceiveLib]
    end

    subgraph Phase5["阶段 5: 测试"]
        E1[小额测试 10 USDT]
        E2[验证目标链收款]
        E3[测试反向跨链]
        E4[压力测试]
    end

    A1 --> A2 --> A3 --> A4
    A4 --> B1
    B1 --> B2 --> B3
    B3 --> C1
    C1 --> C2 --> C3
    C3 --> D1
    D1 --> D2 --> D3 --> D4
    D4 --> E1
    E1 --> E2 --> E3 --> E4
```

---

## 2.7 Hardhat 配置

```typescript
// hardhat.config.ts
import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.20",
    settings: { optimizer: { enabled: true, runs: 200 } },
  },
  networks: {
    ethereum: {
      url: process.env.ETH_RPC_URL,
      accounts: [process.env.DEPLOYER_PRIVATE_KEY!],
      chainId: 1,
    },
    conflux: {
      url: process.env.CFX_RPC_URL,
      accounts: [process.env.DEPLOYER_PRIVATE_KEY!],
      chainId: 1030,
    },
    arbitrum: {
      url: process.env.ARB_RPC_URL,
      accounts: [process.env.DEPLOYER_PRIVATE_KEY!],
      chainId: 42161,
    },
  },
};

export default config;
```

---

## 2.8 部署脚本

```typescript
// scripts/deploy.ts
import { ethers } from "hardhat";

const LZ_ENDPOINTS = {
  ethereum: "0x1a44076050125825900e736c501f859c50fE728c",
  conflux: "0x...",
  arbitrum: "0x1a44076050125825900e736c501f859c50fE728c",
};

const EID = {
  ethereum: 30101,
  conflux: 30250,
  arbitrum: 30110,
};

const USDT_ADDRESS = {
  ethereum: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
  arbitrum: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9",
};

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log("Deploying with:", deployer.address);

  const network = process.env.NETWORK || "ethereum";
  
  if (network === "ethereum" || network === "arbitrum") {
    await deployOFTAdapter(network, deployer);
  } else if (network === "conflux") {
    await deployConfluxContracts(deployer);
  }
}

async function deployOFTAdapter(network: string, deployer: any) {
  const OFTAdapter = await ethers.getContractFactory("USDTOFTAdapter");
  const adapter = await OFTAdapter.deploy(
    USDT_ADDRESS[network],
    LZ_ENDPOINTS[network],
    deployer.address
  );
  await adapter.waitForDeployment();
  console.log("OFTAdapter:", await adapter.getAddress());
}

async function deployConfluxContracts(deployer: any) {
  // 部署 OFT
  const OFT = await ethers.getContractFactory("USDT0");
  const oft = await OFT.deploy("USDT0", "USDT0", LZ_ENDPOINTS.conflux, deployer.address);
  await oft.waitForDeployment();
  console.log("USDT0:", await oft.getAddress());
  
  // 部署 DVN
  const dvnSigners = [
    process.env.DVN_SIGNER_1!,
    process.env.DVN_SIGNER_2!,
    process.env.DVN_SIGNER_3!,
  ];
  
  const CustomDVN = await ethers.getContractFactory("CustomDVN");
  const dvn = await CustomDVN.deploy(dvnSigners, 2, deployer.address);
  await dvn.waitForDeployment();
  console.log("CustomDVN:", await dvn.getAddress());
}

main().catch(console.error);
```

---

## 2.9 合约地址汇总

| 网络 | 合约 | 地址 |
|------|------|------|
| Ethereum | OFTAdapter | `0x...` |
| Ethereum | Endpoint | `0x1a44076050125825900e736c501f859c50fE728c` |
| Arbitrum | OFTAdapter | `0x...` |
| Conflux | USDT0 | `0x...` |
| Conflux | CustomDVN | `0x...` |
| Conflux | Endpoint | `0x...` |

---

# 3. DVN 节点部署

## 3.1 DVN 节点架构

```mermaid
flowchart TB
    subgraph DVNNode["DVN 节点"]
        subgraph Services["核心服务"]
            Listener[事件监听服务<br/>Event Listener]
            Verifier[交易验证服务<br/>Verifier]
            Signer[签名服务<br/>Signer]
            Submitter[提交服务<br/>Submitter]
        end

        subgraph Queue["消息队列"]
            Redis[(Redis)]
        end

        subgraph Storage["持久化存储"]
            Postgres[(PostgreSQL<br/>审计日志)]
        end
    end

    subgraph External["外部依赖"]
        ETH[以太坊节点]
        CFX[Conflux 节点]
        HSM[CloudHSM]
    end

    ETH --> Listener
    CFX --> Listener
    Listener --> Redis
    Redis --> Verifier
    Verifier --> Redis
    Redis --> Signer
    Signer <--> HSM
    Signer --> Redis
    Redis --> Submitter
    Submitter --> CFX

    Listener --> Postgres
    Verifier --> Postgres
    Signer --> Postgres
    Submitter --> Postgres
```

---

## 3.2 多区域部署架构

```mermaid
flowchart TB
    subgraph AWS["AWS 美东 us-east-1"]
        subgraph AWSVPC["VPC 10.0.0.0/16"]
            subgraph AWSPrivate["私有子网"]
                DVN1[DVN Node #1]
                ETH1[Geth Node]
                CFX1[Conflux Node]
                HSM1[CloudHSM]
            end
            AWSNAT[NAT Gateway]
        end
    end

    subgraph Aliyun["阿里云 杭州"]
        subgraph AliVPC["VPC 10.1.0.0/16"]
            subgraph AliPrivate["私有子网"]
                DVN2[DVN Node #2]
                ETH2[Geth Node]
                CFX2[Conflux Node]
                HSM2[密钥管理服务]
            end
            AliNAT[NAT 网关]
        end
    end

    subgraph GCP["Google Cloud 东京"]
        subgraph GCPVPC["VPC 10.2.0.0/16"]
            subgraph GCPPrivate["私有子网"]
                DVN3[DVN Node #3]
                ETH3[Geth Node]
                CFX3[Conflux Node]
                HSM3[Cloud HSM]
            end
            GCPNAT[Cloud NAT]
        end
    end

    subgraph CrossCloud["跨云连接"]
        VPN1[AWS-阿里云 VPN]
        VPN2[阿里云-GCP VPN]
        VPN3[GCP-AWS VPN]
    end

    AWSVPC <-->|VPN| VPN1
    AliVPC <-->|VPN| VPN1
    AliVPC <-->|VPN| VPN2
    GCPVPC <-->|VPN| VPN2
    GCPVPC <-->|VPN| VPN3
    AWSVPC <-->|VPN| VPN3
```

---

## 3.3 配置文件

```yaml
# config.yaml
service:
  name: dvn-node-1
  region: aws-us-east-1
  log_level: info

# 源链配置
source_chains:
  ethereum:
    chain_id: 1
    eid: 30101
    rpc_url: "http://10.0.2.100:8545"  # 内网 Geth 节点
    confirmations: 12
    block_time: 12s
    contracts:
      endpoint: "0x1a44076050125825900e736c501f859c50fE728c"
      send_lib: "0x..."
      
  arbitrum:
    chain_id: 42161
    eid: 30110
    rpc_url: "http://10.0.2.101:8545"
    confirmations: 64
    block_time: 250ms

# 目标链配置
target_chains:
  conflux:
    chain_id: 1030
    eid: 30250
    rpc_url: "http://10.0.2.102:8545"  # 内网 Conflux 节点
    contracts:
      endpoint: "0x..."
      receive_lib: "0x..."
      dvn: "0x..."

# HSM 配置
hsm:
  provider: aws  # aws | aliyun | gcp
  aws:
    cluster_id: "cluster-xxx"
    hsm_ip: "10.0.2.200"
    key_label: "dvn-signing-key"
    pin_env: "HSM_PIN"

# Redis 配置
redis:
  addr: "10.0.2.50:6379"
  password_env: "REDIS_PASSWORD"
  db: 0

# PostgreSQL 配置
postgres:
  host: "10.0.2.51"
  port: 5432
  database: "dvn"
  user: "dvn"
  password_env: "POSTGRES_PASSWORD"

# 签名配置
signing:
  quorum: 2
  total_signers: 3
  timeout: 30s
```

---

## 3.4 Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  dvn:
    build: .
    container_name: dvn-node
    restart: unless-stopped
    environment:
      - HSM_PIN=${HSM_PIN}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - /opt/cloudhsm:/opt/cloudhsm:ro
    networks:
      - dvn-network
    depends_on:
      - redis
      - postgres

  redis:
    image: redis:7-alpine
    container_name: dvn-redis
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis-data:/data
    networks:
      - dvn-network

  postgres:
    image: postgres:15-alpine
    container_name: dvn-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_USER=dvn
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=dvn
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - dvn-network

networks:
  dvn-network:
    driver: bridge

volumes:
  redis-data:
  postgres-data:
```

---

# 4. 区块链节点部署

## 4.1 节点部署架构

```mermaid
flowchart TB
    subgraph NodeCluster["区块链节点集群"]
        subgraph AWS["AWS 美东"]
            ETH1[Geth 节点<br/>i3.2xlarge<br/>2TB NVMe]
            CFX1[Conflux 节点<br/>m5.xlarge<br/>500GB SSD]
        end
        
        subgraph Aliyun["阿里云 杭州"]
            ETH2[Geth 节点<br/>ecs.i2.2xlarge<br/>2TB NVMe]
            CFX2[Conflux 节点<br/>ecs.g6.xlarge<br/>500GB SSD]
        end
        
        subgraph GCP["Google Cloud 东京"]
            ETH3[Geth 节点<br/>n2-highmem-8<br/>2TB SSD]
            CFX3[Conflux 节点<br/>e2-standard-4<br/>500GB SSD]
        end
    end

    subgraph P2P["P2P 网络"]
        ETHNet[以太坊网络]
        CFXNet[Conflux 网络]
    end

    ETH1 <--> ETHNet
    ETH2 <--> ETHNet
    ETH3 <--> ETHNet
    
    CFX1 <--> CFXNet
    CFX2 <--> CFXNet
    CFX3 <--> CFXNet
```

---

## 4.2 硬件要求

### 以太坊节点

| 配置项 | 最低要求 | 推荐配置 |
|--------|---------|---------|
| CPU | 4 核 | 8 核 |
| 内存 | 16 GB | 32 GB |
| 存储 | 1 TB SSD | 2 TB NVMe |
| IOPS | 10,000 | 16,000+ |
| 带宽 | 25 Mbps | 100 Mbps |
| 同步时间 | 3-5 天 | 1-2 天 |

### Conflux 节点

| 配置项 | 最低要求 | 推荐配置 |
|--------|---------|---------|
| CPU | 4 核 | 8 核 |
| 内存 | 8 GB | 16 GB |
| 存储 | 200 GB SSD | 500 GB SSD |
| 带宽 | 10 Mbps | 50 Mbps |
| 同步时间 | 6-12 小时 | 3-6 小时 |

---

## 4.3 节点高可用配置

```mermaid
flowchart TB
    subgraph LB["负载均衡层"]
        HAProxy[HAProxy]
    end
    
    subgraph Primary["主节点"]
        ETH_Primary[Geth Primary]
        CFX_Primary[Conflux Primary]
    end
    
    subgraph Standby["备用节点"]
        ETH_Standby[Geth Standby]
        CFX_Standby[Conflux Standby]
    end
    
    subgraph HealthCheck["健康检查"]
        HC[Health Checker]
    end
    
    HAProxy --> ETH_Primary
    HAProxy --> ETH_Standby
    HAProxy --> CFX_Primary
    HAProxy --> CFX_Standby
    
    HC --> ETH_Primary
    HC --> ETH_Standby
    HC --> CFX_Primary
    HC --> CFX_Standby
    HC --> HAProxy
```

---

## 4.4 监控指标

| 指标 | 描述 | 告警阈值 |
|------|------|---------|
| `node_sync_status` | 同步状态 | syncing = true |
| `node_block_height` | 区块高度 | 落后 > 10 块 |
| `node_peer_count` | 对等节点数 | < 5 |
| `node_disk_usage` | 磁盘使用率 | > 85% |
| `node_rpc_latency` | RPC 延迟 | > 500ms |
| `node_rpc_errors` | RPC 错误数 | > 10/分钟 |

---

# 5. HSM 多云部署

## 5.1 多云 HSM 架构

```mermaid
flowchart TB
    subgraph MultiCloud["多云 HSM 架构"]
        subgraph AWS["AWS CloudHSM"]
            AWSHSM[CloudHSM 集群]
            AWSKey[密钥 #1<br/>secp256k1]
        end
        
        subgraph Aliyun["阿里云密钥管理"]
            AliKMS[密钥管理服务 KMS]
            AliKey[密钥 #2<br/>secp256k1]
        end
        
        subgraph GCP["Google Cloud HSM"]
            GCPHSM[Cloud HSM]
            GCPKey[密钥 #3<br/>secp256k1]
        end
    end
    
    subgraph DVNNodes["DVN 节点"]
        DVN1[DVN #1] --> AWSHSM
        DVN2[DVN #2] --> AliKMS
        DVN3[DVN #3] --> GCPHSM
    end
    
    subgraph Signing["签名策略"]
        Threshold[阈值签名<br/>2/3 必须签名]
    end
    
    AWSHSM --> Threshold
    AliKMS --> Threshold
    GCPHSM --> Threshold
```

---

## 5.2 统一签名接口

```mermaid
classDiagram
    class Signer {
        <<interface>>
        +sign(message bytes) bytes
        +getPublicKey() bytes
        +getAddress() string
    }
    
    class AWSCloudHSMSigner {
        -pkcs11Ctx
        -session
        -keyHandle
        +sign(message bytes) bytes
        +getPublicKey() bytes
    }
    
    class AliyunKMSSigner {
        -client
        -keyId
        +sign(message bytes) bytes
        +getPublicKey() bytes
    }
    
    class GCPKMSSigner {
        -client
        -keyVersionName
        +sign(message bytes) bytes
        +getPublicKey() bytes
    }
    
    Signer <|.. AWSCloudHSMSigner
    Signer <|.. AliyunKMSSigner
    Signer <|.. GCPKMSSigner
```

---

## 5.3 密钥备份策略

```mermaid
flowchart TB
    subgraph Backup["密钥备份策略"]
        subgraph AWS["AWS CloudHSM"]
            AWSKey[主密钥]
            AWSBackup[HSM 集群自动备份]
        end
        
        subgraph Aliyun["阿里云 KMS"]
            AliKey[主密钥]
            AliBackup[自动备份到 OSS]
        end
        
        subgraph GCP["Google Cloud"]
            GCPKey[主密钥]
            GCPBackup[自动多区域复制]
        end
        
        subgraph Offline["离线备份"]
            Paper[纸质备份<br/>保险箱存储]
            Split[Shamir 秘密分割<br/>3/5 方案]
        end
    end
    
    AWSKey --> AWSBackup
    AliKey --> AliBackup
    GCPKey --> GCPBackup
    
    AWSBackup -.->|定期| Offline
    AliBackup -.->|定期| Offline
    GCPBackup -.->|定期| Offline
```

---

## 5.4 故障恢复流程

```mermaid
flowchart TD
    Start[检测到 HSM 故障]
    
    Start --> Check{哪个云?}
    
    Check -->|AWS| AWS1[检查 CloudHSM 状态]
    Check -->|阿里云| Ali1[检查 KMS 状态]
    Check -->|GCP| GCP1[检查 Cloud HSM 状态]
    
    AWS1 --> AWS2{可恢复?}
    AWS2 -->|是| AWS3[重启 HSM 实例]
    AWS2 -->|否| AWS4[从备份恢复]
    
    Ali1 --> Ali2{可恢复?}
    Ali2 -->|是| Ali3[刷新密钥服务]
    Ali2 -->|否| Ali4[联系阿里云支持]
    
    GCP1 --> GCP2{可恢复?}
    GCP2 -->|是| GCP3[重新创建密钥版本]
    GCP2 -->|否| GCP4[从备份恢复]
    
    AWS3 --> Verify[验证签名功能]
    AWS4 --> Verify
    Ali3 --> Verify
    Ali4 --> Verify
    GCP3 --> Verify
    GCP4 --> Verify
    
    Verify --> Done[恢复完成]
```

---

# 6. 网络架构

## 6.1 整体网络架构

```mermaid
flowchart TB
    subgraph Internet["互联网"]
        P2P[区块链 P2P 网络]
    end

    subgraph AWS["AWS us-east-1"]
        subgraph AWSVPC["VPC 10.0.0.0/16"]
            subgraph AWSPublic["公有子网 10.0.1.0/24"]
                AWSNAT[NAT Gateway]
                AWSBastion[堡垒机]
            end
            subgraph AWSPrivate["私有子网 10.0.2.0/24"]
                AWSDVN[DVN Node]
                AWSETH[Geth Node]
                AWSCFX[Conflux Node]
                AWSHSM[CloudHSM]
            end
        end
    end

    subgraph Aliyun["阿里云 杭州"]
        subgraph AliVPC["VPC 10.1.0.0/16"]
            subgraph AliPublic["公网子网 10.1.1.0/24"]
                AliNAT[NAT 网关]
                AliBastion[堡垒机]
            end
            subgraph AliPrivate["内网子网 10.1.2.0/24"]
                AliDVN[DVN Node]
                AliETH[Geth Node]
                AliCFX[Conflux Node]
                AliHSM[KMS]
            end
        end
    end

    subgraph GCP["Google Cloud 东京"]
        subgraph GCPVPC["VPC 10.2.0.0/16"]
            subgraph GCPPublic["公有子网 10.2.1.0/24"]
                GCPNAT[Cloud NAT]
                GCPBastion[堡垒机]
            end
            subgraph GCPPrivate["私有子网 10.2.2.0/24"]
                GCPDVN[DVN Node]
                GCPETH[Geth Node]
                GCPCFX[Conflux Node]
                GCPHSM[Cloud HSM]
            end
        end
    end

    subgraph CrossCloud["跨云连接"]
        VPN12[AWS-阿里云 VPN<br/>IPSec]
        VPN23[阿里云-GCP VPN<br/>IPSec]
        VPN13[AWS-GCP VPN<br/>IPSec]
    end

    AWSNAT --> P2P
    AliNAT --> P2P
    GCPNAT --> P2P

    AWSVPC <--> VPN12 <--> AliVPC
    AliVPC <--> VPN23 <--> GCPVPC
    AWSVPC <--> VPN13 <--> GCPVPC
```

---

## 6.2 跨云 VPN 配置

```mermaid
flowchart LR
    subgraph AWS["AWS 10.0.0.0/16"]
        AWSVGW[VPN Gateway<br/>203.0.113.10]
    end

    subgraph Aliyun["阿里云 10.1.0.0/16"]
        AliVPN[VPN 网关<br/>203.0.113.20]
    end

    subgraph GCP["GCP 10.2.0.0/16"]
        GCPVPN[Cloud VPN<br/>203.0.113.30]
    end

    AWSVGW <-->|IPSec Tunnel 1| AliVPN
    AliVPN <-->|IPSec Tunnel 2| GCPVPN
    GCPVPN <-->|IPSec Tunnel 3| AWSVGW
```

---

## 6.3 IP 地址规划

| 区域 | 网段 | 用途 |
|------|------|------|
| **AWS** | 10.0.0.0/16 | |
| | 10.0.1.0/24 | 公有子网 (NAT, 堡垒机) |
| | 10.0.2.0/24 | 私有子网 (DVN, 节点, HSM) |
| **阿里云** | 10.1.0.0/16 | |
| | 10.1.1.0/24 | 公网子网 |
| | 10.1.2.0/24 | 内网子网 |
| **GCP** | 10.2.0.0/16 | |
| | 10.2.1.0/24 | 公有子网 |
| | 10.2.2.0/24 | 私有子网 |

---

## 6.4 网络安全检查清单

| 检查项 | 状态 | 说明 |
|--------|:----:|------|
| DVN 节点无公网 IP | ✅ | 部署在私有子网 |
| 入站流量限制 | ✅ | 只允许内网访问 |
| HSM 访问限制 | ✅ | 只允许 DVN 安全组 |
| RPC 端口限制 | ✅ | 只允许 DVN 安全组 |
| VPN 加密 | ✅ | IPSec AES-256 |
| 堡垒机访问限制 | ✅ | 只允许管理员 IP |
| 流量日志 | ✅ | VPC Flow Logs 启用 |

---

# 7. 监控告警

## 7.1 监控架构

```mermaid
flowchart TB
    subgraph DataSources["数据源"]
        DVN[DVN 节点<br/>:8080/metrics]
        Geth[Geth 节点<br/>:6060/metrics]
        Conflux[Conflux 节点<br/>:8080/metrics]
        Redis[Redis<br/>:9121/metrics]
        Postgres[PostgreSQL<br/>:9187/metrics]
    end

    subgraph Collection["采集层"]
        Prometheus[Prometheus<br/>时序数据库]
        Loki[Loki<br/>日志聚合]
    end

    subgraph Visualization["可视化"]
        Grafana[Grafana<br/>仪表盘]
    end

    subgraph Alerting["告警"]
        AlertManager[AlertManager]
        PagerDuty[PagerDuty]
        Slack[Slack]
        WeChat[企业微信]
    end

    DVN --> Prometheus
    Geth --> Prometheus
    Conflux --> Prometheus
    Redis --> Prometheus
    Postgres --> Prometheus

    DVN --> Loki
    Geth --> Loki
    Conflux --> Loki

    Prometheus --> Grafana
    Loki --> Grafana
    Prometheus --> AlertManager

    AlertManager --> PagerDuty
    AlertManager --> Slack
    AlertManager --> WeChat
```

---

## 7.2 核心监控指标

### DVN 服务指标

| 指标名称 | 类型 | 描述 | 告警阈值 |
|---------|------|------|---------|
| `dvn_events_received_total` | Counter | 接收的事件总数 | - |
| `dvn_events_processed_total` | Counter | 处理的事件总数 | - |
| `dvn_events_pending` | Gauge | 待处理事件数 | > 100 |
| `dvn_verification_latency_seconds` | Histogram | 验证延迟 | p99 > 30s |
| `dvn_signing_duration_seconds` | Histogram | 签名耗时 | p99 > 5s |
| `dvn_signing_errors_total` | Counter | 签名错误数 | > 0 |
| `dvn_submission_success_total` | Counter | 成功提交数 | - |
| `dvn_submission_failed_total` | Counter | 失败提交数 | > 0 |

### 区块链节点指标

| 指标名称 | 描述 | 告警阈值 |
|---------|------|---------|
| `eth_syncing` | 同步状态 | true |
| `eth_block_number` | 当前区块高度 | 落后 > 10 块 |
| `eth_peer_count` | 对等节点数 | < 5 |
| `eth_rpc_latency_seconds` | RPC 延迟 | > 500ms |
| `eth_rpc_errors_total` | RPC 错误数 | > 10/min |

---

## 7.3 告警响应流程

```mermaid
flowchart TD
    Alert[告警触发]
    
    Alert --> Severity{严重级别}
    
    Severity -->|Critical| Critical[立即响应]
    Severity -->|Warning| Warning[15分钟内响应]
    Severity -->|Info| Info[工作时间处理]
    
    Critical --> Oncall[通知值班人员]
    Oncall --> Ack[确认告警]
    Ack --> Diagnose[诊断问题]
    Diagnose --> Fix[修复问题]
    Fix --> Verify[验证修复]
    Verify --> Resolve[解决告警]
    Resolve --> Postmortem[事后分析]
    
    Warning --> Check[检查问题]
    Check --> Minor{需要修复?}
    Minor -->|是| Fix
    Minor -->|否| Monitor[持续监控]
    
    Info --> Log[记录日志]
```

---

# 8. 运维手册

## 8.1 日常运维流程

```mermaid
flowchart LR
    subgraph Daily["每日任务"]
        D1[检查服务状态]
        D2[查看监控仪表盘]
        D3[检查告警日志]
        D4[验证跨链功能]
    end

    subgraph Weekly["每周任务"]
        W1[检查磁盘空间]
        W2[审查安全日志]
        W3[更新系统补丁]
        W4[备份验证]
    end

    subgraph Monthly["每月任务"]
        M1[性能分析]
        M2[成本审计]
        M3[灾难恢复演练]
        M4[密钥轮换评估]
    end

    D1 --> D2 --> D3 --> D4
    W1 --> W2 --> W3 --> W4
    M1 --> M2 --> M3 --> M4
```

---

## 8.2 服务管理命令

```bash
# DVN 服务
docker-compose ps           # 查看服务状态
docker-compose logs -f dvn  # 查看日志
docker-compose restart dvn  # 重启服务

# 区块链节点
curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' \
    http://localhost:8545 | jq

# HSM 管理
/opt/cloudhsm/bin/cloudhsm-cli cluster describe
```

---

## 8.3 故障排查决策树

```mermaid
flowchart TD
    Start[发现问题]
    
    Start --> Check1{服务是否运行?}
    
    Check1 -->|否| Action1[启动服务]
    Check1 -->|是| Check2{日志有错误?}
    
    Check2 -->|是| Analyze[分析错误日志]
    Check2 -->|否| Check3{网络连通?}
    
    Analyze --> ErrorType{错误类型}
    ErrorType -->|HSM 错误| HSMFix[检查 HSM 连接]
    ErrorType -->|RPC 错误| RPCFix[检查区块链节点]
    ErrorType -->|签名错误| SignFix[检查密钥配置]
    ErrorType -->|其他| General[通用排查]
    
    Check3 -->|否| NetFix[检查网络配置]
    Check3 -->|是| Check4{资源充足?}
    
    Check4 -->|否| ResourceFix[扩容/清理]
    Check4 -->|是| Escalate[升级处理]
    
    Action1 --> Verify[验证服务]
    HSMFix --> Verify
    RPCFix --> Verify
    SignFix --> Verify
    NetFix --> Verify
    ResourceFix --> Verify
    General --> Verify
    
    Verify --> Done[问题解决]
```

---

## 8.4 紧急响应级别

| 级别 | 描述 | 响应时间 | 示例 |
|------|------|---------|------|
| P0 | 系统完全不可用 | 15 分钟 | 所有 DVN 宕机 |
| P1 | 核心功能受损 | 30 分钟 | 无法提交验证 |
| P2 | 部分功能受损 | 2 小时 | 单个区域故障 |
| P3 | 性能下降 | 24 小时 | 延迟增加 |

---

## 8.5 备份策略

```mermaid
flowchart TB
    subgraph Backup["备份内容"]
        DB[(PostgreSQL<br/>审计日志)]
        Config[配置文件]
        HSMKey[HSM 密钥<br/>云服务商托管]
    end

    subgraph Schedule["备份频率"]
        Daily[每日 02:00]
        Weekly[每周日 03:00]
        Monthly[每月 1 日 04:00]
    end

    subgraph Storage["存储位置"]
        S3[AWS S3]
        OSS[阿里云 OSS]
        GCS[Google Cloud Storage]
    end

    DB --> Daily
    Config --> Weekly
    
    Daily --> S3
    Daily --> OSS
    Weekly --> GCS
```

---

## 8.6 运维检查表

### 每日检查

| 检查项 | 命令/方法 | 预期结果 |
|--------|----------|---------|
| DVN 服务状态 | `docker-compose ps` | 3 个 running |
| 事件积压 | Grafana 仪表盘 | < 10 |
| 签名延迟 | Grafana 仪表盘 | p99 < 5s |
| 区块同步 | `eth_syncing` | false |
| HSM 连接 | `cloudhsm-cli cluster describe` | connected |
| 磁盘使用 | `df -h` | < 80% |
| 告警数量 | AlertManager | 0 active |

### 每周检查

| 检查项 | 命令/方法 | 预期结果 |
|--------|----------|---------|
| 安全日志审查 | CloudTrail / 操作审计 | 无异常 |
| 备份验证 | 恢复测试 | 成功 |
| 系统更新 | `yum check-update` | 评估并更新 |
| 证书有效期 | `openssl x509 -enddate` | > 30 天 |
| 跨云连接 | VPN 状态检查 | 3 条 tunnel up |

---

## 📅 版本历史

| 版本 | 日期 | 更新内容 |
|------|------|---------|
| v1.0.0 | 2024-01 | 初始版本 |

