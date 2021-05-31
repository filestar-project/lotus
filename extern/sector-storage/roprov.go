package sectorstorage

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/lotus/extern/sector-storage/stores"
	"github.com/filecoin-project/lotus/extern/sector-storage/storiface"
)

type readonlyProvider struct {
	index stores.SectorIndex
	stor  *stores.Local
	spt   abi.RegisteredSealProof
}

func (l *readonlyProvider) AcquireSector(ctx context.Context, id abi.SectorID, existing storiface.SectorFileType, allocate storiface.SectorFileType, sealing storiface.PathType) (storiface.SectorPaths, func(), error) {
	if allocate != storiface.FTNone {
		return storiface.SectorPaths{}, nil, xerrors.New("read-only storage")
	}

	ssize, err := l.spt.SectorSize()
	if err != nil {
		return storiface.SectorPaths{}, nil, xerrors.Errorf("failed to determine sector size: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	// use TryLock to avoid blocking
	locked, err := l.index.StorageTryLock(ctx, id, existing, storiface.FTNone)
	if err != nil {
		cancel()
		return storiface.SectorPaths{}, nil, xerrors.Errorf("acquiring sector lock: %w", err)
	}
	if !locked {
		cancel()
		return storiface.SectorPaths{}, nil, xerrors.Errorf("failed to acquire sector lock")
	}

	p, _, err := l.stor.AcquireSector(ctx, id, ssize, existing, allocate, sealing, storiface.AcquireMove)

	return p, cancel, err
}
