package currency

import (
	"errors"

	"github.com/vazha/gocryptotrader/exchanges/asset"
)

// GetAssetTypes returns a list of stored asset types
func (p *PairsManager) GetAssetTypes() asset.Items {
	p.m.Lock()
	defer p.m.Unlock()
	var assetTypes asset.Items
	for k := range p.Pairs {
		assetTypes = append(assetTypes, k)
	}
	return assetTypes
}

// Get gets the currency pair config based on the asset type
func (p *PairsManager) Get(a asset.Item) *PairStore {
	p.m.Lock()
	defer p.m.Unlock()
	c, ok := p.Pairs[a]
	if !ok {
		return nil
	}
	return c
}

// Store stores a new currency pair config based on its asset type
func (p *PairsManager) Store(a asset.Item, ps PairStore) {
	p.m.Lock()

	if p.Pairs == nil {
		p.Pairs = make(map[asset.Item]*PairStore)
	}

	if !p.AssetTypes.Contains(a) {
		p.AssetTypes = append(p.AssetTypes, a)
	}

	p.Pairs[a] = &ps
	p.m.Unlock()
}

// Delete deletes a map entry based on the supplied asset type
func (p *PairsManager) Delete(a asset.Item) {
	p.m.Lock()
	defer p.m.Unlock()
	if p.Pairs == nil {
		return
	}

	_, ok := p.Pairs[a]
	if !ok {
		return
	}

	delete(p.Pairs, a)
}

// GetPairs gets a list of stored pairs based on the asset type and whether
// they're enabled or not
func (p *PairsManager) GetPairs(a asset.Item, enabled bool) Pairs {
	p.m.Lock()
	defer p.m.Unlock()
	if p.Pairs == nil {
		return nil
	}

	c, ok := p.Pairs[a]
	if !ok {
		return nil
	}

	var pairs Pairs
	if enabled {
		pairs = c.Enabled
	} else {
		pairs = c.Available
	}

	return pairs
}

// StorePairs stores a list of pairs based on the asset type and whether
// they're enabled or not
func (p *PairsManager) StorePairs(a asset.Item, pairs Pairs, enabled bool) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.Pairs == nil {
		p.Pairs = make(map[asset.Item]*PairStore)
	}

	c, ok := p.Pairs[a]
	if !ok {
		c = new(PairStore)
	}

	if enabled {
		c.Enabled = pairs
	} else {
		c.Available = pairs
	}

	p.Pairs[a] = c
}

// DisablePair removes the pair from the enabled pairs list if found
func (p *PairsManager) DisablePair(a asset.Item, pair Pair) error {
	p.m.Lock()
	defer p.m.Unlock()

	if p.Pairs == nil {
		return errors.New("pair manager not initialised")
	}

	c, ok := p.Pairs[a]
	if !ok {
		return errors.New("asset type not found")
	}

	if c == nil {
		return errors.New("currency store is nil")
	}

	if !c.Enabled.Contains(pair, true) {
		return errors.New("specified pair is not enabled")
	}

	c.Enabled = c.Enabled.Remove(pair)
	return nil
}

// EnablePair adds a pair to the list of enabled pairs if it exists in the list
// of available pairs and isn't already added
func (p *PairsManager) EnablePair(a asset.Item, pair Pair) error {
	p.m.Lock()
	defer p.m.Unlock()

	if p.Pairs == nil {
		return errors.New("pair manager not initialised")
	}

	c, ok := p.Pairs[a]
	if !ok {
		return errors.New("asset type not found")
	}

	if c == nil {
		return errors.New("currency store is nil")
	}

	if !c.Available.Contains(pair, true) {
		return errors.New("specified pair was not found in the list of available pairs")
	}

	if c.Enabled.Contains(pair, true) {
		return errors.New("specified pair is already enabled")
	}

	c.Enabled = c.Enabled.Add(pair)
	return nil
}
