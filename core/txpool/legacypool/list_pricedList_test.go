// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package legacypool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// go test -run ^$ -bench ^BenchmarkPricedList -benchtime 100x github.com/ethereum/go-ethereum/core/txpool/legacypool

var (
	txsCount = DefaultConfig.GlobalSlots + DefaultConfig.GlobalQueue
	key, _   = crypto.GenerateKey()
)

func prepareTxs() types.Transactions {
	txs := make(types.Transactions, txsCount)
	for i := range len(txs) {
		txs[i] = pricedTransaction(uint64(i), 0, big.NewInt(int64(i+1)), key)
	}
	return txs
}

func prepareList(addAll bool) *pricedList {
	txs := prepareTxs()
	list := newPricedList(newLookup())
	for _, tx := range txs {
		list.Put(tx)
		if addAll {
			list.all.Add(tx)
		}
	}
	return list
}

func getFirstTx(txs types.Transactions) *types.Transaction {
	return txs[0]
}

func getLastTx(txs types.Transactions) *types.Transaction {
	return txs[len(txs)-1]
}

func BenchmarkPricedListPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txs := prepareTxs()
		list := newPricedList(newLookup())
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, 0, len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 0, len(list.all.txs))
		for _, tx := range txs {
			b.StartTimer()
			list.Put(tx)
			b.StopTimer()
		}
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list)) // updated to put
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 0, len(list.all.txs))
	}
}

func BenchmarkPricedListUnderpricedTrueFastestCase(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(false)
		firstTx := getFirstTx(list.urgent.list)
		list.all.Add(firstTx)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 1, len(list.all.txs))
		b.StartTimer()
		result := list.Underpriced(firstTx)
		b.StopTimer()
		assert.True(b, result)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 1, len(list.all.txs))
	}
}

func BenchmarkPricedListUnderpricedTrueWorstCase(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(false)
		lastTx := getLastTx(list.urgent.list)
		list.all.Add(lastTx)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 1, len(list.all.txs))
		tx := pricedTransaction(lastTx.Nonce()+1, lastTx.Gas(), big.NewInt(0), key)
		b.StartTimer()
		result := list.Underpriced(tx)
		b.StopTimer()
		assert.True(b, result)
		assert.Equal(b, int64(-txsCount+1), list.stales.Load()) // updated since not to get from all except once: stales.Add(-1)
		assert.Equal(b, 1, len(list.urgent.list))               // updated since not to get from all except once: heap.Pop(urgent)
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 1, len(list.all.txs))
	}
}

func BenchmarkPricedListDiscardTrueDropsLenTxs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(true)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, int(txsCount), len(list.all.txs))
		slots := int(txsCount)
		b.StartTimer()
		drops, result := list.Discard(slots)
		b.StopTimer()
		assert.Equal(b, slots, len(drops)) // will be removed txs from outside of the Discard function
		assert.True(b, result)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, 0, len(list.urgent.list)) // updated to return drops: heap.Pop(urgent)
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, int(txsCount), len(list.all.txs))
		list.all.Clear()
		list.Reheap()
	}
}

func BenchmarkPricedListDiscardFalseDropsLenTxs0(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(false)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 0, len(list.all.txs))
		slots := int(txsCount)
		b.StartTimer()
		drops, result := list.Discard(slots)
		b.StopTimer()
		assert.Equal(b, 0, len(drops)) // drops are empty since not to get from all
		assert.False(b, result)
		assert.Equal(b, int64(-txsCount), list.stales.Load()) // updated since not to get from all: stales.Add(-1)
		assert.Equal(b, 0, len(list.urgent.list))             // updated since not to get from all: heap.Pop(urgent)
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, 0, len(list.all.txs))
		list.all.Clear()
		list.Reheap()
	}
}

func BenchmarkPricedListDiscardFalseDropsLenTxsMinus1(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(true)
		firstTx := getFirstTx(list.urgent.list)
		list.all.Remove(firstTx.Hash())
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, int(txsCount), len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, int(txsCount-1), len(list.all.txs))
		slots := int(txsCount)
		b.StartTimer()
		drops, result := list.Discard(slots)
		b.StopTimer()
		assert.Equal(b, 0, len(drops)) // drops are added again in the Discard function, except for one: heap.Push(urgent)
		assert.False(b, result)
		assert.Equal(b, int64(-1), list.stales.Load())          // updated since not to get from all once: stales.Add(-1)
		assert.Equal(b, int(txsCount-1), len(list.urgent.list)) // updated since not to get from all once: heap.Pop(urgent)
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, int(txsCount-1), len(list.all.txs))
		list.all.Clear()
		list.Reheap()
	}
}

func BenchmarkPricedListReheap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := prepareList(true)
		slots := int(txsCount)
		list.Discard(slots)
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, 0, len(list.urgent.list))
		assert.Equal(b, 0, len(list.floating.list))
		assert.Equal(b, int(txsCount), len(list.all.txs))
		b.StartTimer()
		list.Reheap()
		b.StopTimer()
		assert.Equal(b, int64(0), list.stales.Load())
		assert.Equal(b, 4916, len(list.urgent.list))   // updated to set 80% of all txs
		assert.Equal(b, 1228, len(list.floating.list)) // updated to set 20% of all txs
		assert.Equal(b, int(txsCount), len(list.all.txs))
		list.all.Clear()
		list.Reheap()
	}
}
