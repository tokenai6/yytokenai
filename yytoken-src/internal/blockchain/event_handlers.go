package blockchain

import (
	"XWFrame/internal/frame/consts"
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// StakeEventHandler 质押事件处理器
type StakeEventHandler struct {
	listener *StakeEventListener
}

// NewStakeEventHandler 创建质押事件处理器
func NewStakeEventHandler(listener *StakeEventListener) *StakeEventHandler {
	return &StakeEventHandler{
		listener: listener,
	}
}

func (h *StakeEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *StakeEventHandler) GetContractAddress() common.Address {
	// 返回普通质押合约地址（组合质押事件会通过事件ID匹配）
	return h.listener.contractAddress
}

func (h *StakeEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processStakeEvent(ctx, event)
}

func (h *StakeEventHandler) GetName() string {
	return "质押事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *StakeEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameStake
}

// BurnEventHandler Swap销毁事件处理器
type BurnEventHandler struct {
	listener *BurnEventListener
}

// NewBurnEventHandler 创建Swap销毁事件处理器
func NewBurnEventHandler(listener *BurnEventListener) *BurnEventHandler {
	return &BurnEventHandler{
		listener: listener,
	}
}

func (h *BurnEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *BurnEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *BurnEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processBurnEvent(ctx, event)
}

func (h *BurnEventHandler) GetName() string {
	return "Swap销毁事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *BurnEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameBurn
}

// NodeDividendUsdtEventHandler 节点分红USDT转账事件处理器
type NodeDividendUsdtEventHandler struct {
	listener *NodeDividendUsdtListener
}

// NewNodeDividendUsdtEventHandler 创建节点分红USDT转账事件处理器
func NewNodeDividendUsdtEventHandler(listener *NodeDividendUsdtListener) *NodeDividendUsdtEventHandler {
	return &NodeDividendUsdtEventHandler{
		listener: listener,
	}
}

func (h *NodeDividendUsdtEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *NodeDividendUsdtEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *NodeDividendUsdtEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processTransferEvent(ctx, event)
}

func (h *NodeDividendUsdtEventHandler) GetName() string {
	return "节点分红USDT转账事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *NodeDividendUsdtEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameNodeDividendUsdt
}

// CombinationStakeEventHandler 组合质押事件处理器（已合并到StakeEventListener，保留用于兼容）
// 注意：组合质押事件现在由 StakeEventListener 统一处理
type CombinationStakeEventHandler struct {
	stakeListener *StakeEventListener // 使用统一的质押监听器
}

// NewCombinationStakeEventHandler 创建组合质押事件处理器（委托给StakeEventListener）
func NewCombinationStakeEventHandler(stakeListener *StakeEventListener) *CombinationStakeEventHandler {
	return &CombinationStakeEventHandler{
		stakeListener: stakeListener,
	}
}

func (h *CombinationStakeEventHandler) GetEventID() common.Hash {
	return h.stakeListener.combinationEventID
}

func (h *CombinationStakeEventHandler) GetContractAddress() common.Address {
	return h.stakeListener.combinationContractAddr
}

func (h *CombinationStakeEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	// 委托给统一的质押监听器处理
	return h.stakeListener.processStakeEvent(ctx, event)
}

func (h *CombinationStakeEventHandler) GetName() string {
	return "组合质押事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *CombinationStakeEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameCombinationStake
}

// MintStakeEventHandler APG Mint质押事件处理器
type MintStakeEventHandler struct {
	listener *MintStakeEventListener
}

// NewMintStakeEventHandler 创建APG Mint质押事件处理器
func NewMintStakeEventHandler(listener *MintStakeEventListener) *MintStakeEventHandler {
	return &MintStakeEventHandler{
		listener: listener,
	}
}

func (h *MintStakeEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *MintStakeEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *MintStakeEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processMintStakeEvent(ctx, event)
}

func (h *MintStakeEventHandler) GetName() string {
	return "APG Mint质押事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *MintStakeEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameMintStake
}

// ApgGroupEventHandler APG拼团事件处理器
type ApgGroupEventHandler struct {
	listener *ApgGroupEventListener
}

// NewApgGroupEventHandler 创建APG拼团事件处理器
func NewApgGroupEventHandler(listener *ApgGroupEventListener) *ApgGroupEventHandler {
	return &ApgGroupEventHandler{
		listener: listener,
	}
}

func (h *ApgGroupEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *ApgGroupEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *ApgGroupEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processApgGroupEvent(ctx, event)
}

func (h *ApgGroupEventHandler) GetName() string {
	return "APG拼团事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *ApgGroupEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameApgGroup
}

// ApgRefundEventHandler APG领取/退款事件处理器
type ApgRefundEventHandler struct {
	listener *ApgRefundEventListener
}

// NewApgRefundEventHandler 创建APG领取/退款事件处理器
func NewApgRefundEventHandler(listener *ApgRefundEventListener) *ApgRefundEventHandler {
	return &ApgRefundEventHandler{
		listener: listener,
	}
}

func (h *ApgRefundEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *ApgRefundEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *ApgRefundEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processApgRefundEvent(ctx, event)
}

func (h *ApgRefundEventHandler) GetName() string {
	return "APG领取/退款事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *ApgRefundEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameApgRefund
}

// ApgReferralRewardEventHandler APG推荐奖励领取事件处理器
type ApgReferralRewardEventHandler struct {
	listener *ApgReferralRewardEventListener
}

// NewApgReferralRewardEventHandler 创建APG推荐奖励领取事件处理器
func NewApgReferralRewardEventHandler(listener *ApgReferralRewardEventListener) *ApgReferralRewardEventHandler {
	return &ApgReferralRewardEventHandler{
		listener: listener,
	}
}

func (h *ApgReferralRewardEventHandler) GetEventID() common.Hash {
	return h.listener.eventID
}

func (h *ApgReferralRewardEventHandler) GetContractAddress() common.Address {
	return h.listener.contractAddress
}

func (h *ApgReferralRewardEventHandler) ProcessEvent(ctx context.Context, event types.Log) error {
	return h.listener.processApgReferralRewardEvent(ctx, event)
}

func (h *ApgReferralRewardEventHandler) GetName() string {
	return "APG推荐奖励领取事件处理器"
}

// GetHandlerName 获取处理器名称（实现RetryEventHandler接口）
func (h *ApgReferralRewardEventHandler) GetHandlerName() string {
	return consts.BlockchainHandlerNameApgReferralReward
}