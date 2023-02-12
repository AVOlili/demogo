package dirtree

import (
	"context"
	"fmt"
)

const (
	defMaxDepth      = 100    //最大递归查询深度100
	defMaxTotalCount = 100000 //最多十万

	unKnown = -1

	typeFile   = 1
	typeFolder = 2
)

var type2Str = map[int]string{
	typeFile:   "file",
	typeFolder: "folder",
}

var (
	errDirAlreadyLoaded             = fmt.Errorf("dir already loaded")
	errDirNotLoad                   = fmt.Errorf("dir not load")
	errNoRetrieveNextDepthFilesFunc = fmt.Errorf("no RetrieveNextDepthFilesFunc")
	errNotFolderType                = fmt.Errorf("not folder type")
	errMaxPathDepthLimit            = fmt.Errorf("max path depth limit")
	errFileNumLimit                 = fmt.Errorf("file num list")
	errTotalSizeLimit               = fmt.Errorf("total size limit")
)

type (
	//DirFunc 处理目录的func
	DirFunc func(ctx context.Context, dir *Dir) error
	//RetrieveNextDepthFilesFunc 查找folder下一层子文件(夹)
	RetrieveNextDepthFilesFunc func(ctx context.Context, volumeId, folderId int64) (files, folders []*File, err error)
)

type File struct {
	Id       int64
	ParentId int64 //父目录id
	VolumeId int64 //所属卷id
	Name     string
	Type     int
	Version  int64
	Size     int64
	Ctime    int64
	Creator  int64
	Mtime    int64
	Modifier int64
}

func (r *File) IsFile() bool {
	return r.Type == typeFile
}

func (r *File) IsFolder() bool {
	return r.Type == typeFolder
}

func (r *File) TypeString() string {
	return type2Str[r.Type]
}

type Dir struct {
	originInfo *File   //当前目录的原始信息
	subDirs    []*Dir  //子目录(即子文件夹)
	subFiles   []*File //子文件(纯文件,没有文件夹)
	depth      int64   //当前在目录树中的层级,-1表示未知,0表示树根节点
	count      int64   //当前层级文件(夹)数量,-1表示未知
	size       int64   //当前层级文件大小,-1表示未知
	loaded     bool    //表示当前层级是否已经加载数据
}

/*********************************
注意⚠️:时刻谨记树形结构的节点是基于dir维度的
如:原始文件结构是
	  0(dir)
	  |---10
	  |---11
	  |---12(dir)
	  |		|-----20
	  |		|-----21
	  |		|-----22(dir)
	  |		|		|-----30
	  |		|		|-----31
	  |		|		|-----32
	  |		|		|-----33(dir)
	  |		|		|		|----41
	  |		|-----23(dir)
	  |				|-----34
	  |				|-----35(dir)
	  |				|-----36(dir)
	  |---13(dir)
	  		|-----24(dir)
	  				|-----37(dir)
	  						|---42

然后DFS和BFS遍历树的时侯都是基于Dir的,执行的也是DirFunc
	  0(dir)
	  |---12(dir)
	  |		|-----22(dir)
	  |		|		|-----33(dir)
	  |		|-----23(dir)
	  |				|-----35(dir)
	  |				|-----36(dir)
	  |---13(dir)
	  		|-----24(dir)
	  				|-----37(dir)

*********************************/

func NewDir(folder *File, depth, count, size int64) *Dir {
	if !folder.IsFolder() {
		return nil
	}
	dir := &Dir{
		originInfo: folder,
		subDirs:    nil,
		subFiles:   nil,
		depth:      depth,
		count:      count,
		size:       size,
		loaded:     false,
	}
	return dir
}

/*
	NewVirtualDir
	新建一个不是真实存在的Dir,如根目录
	只在顶层根目录使用,不要在subDirs里面创建VirtualDir。
	如果是虚拟根目录则会过滤掉一些统计,
	比如:虚拟根目录不算入totalCount,GetAllFolders也不会返回虚拟目录
*/
func NewVirtualDir(virtualFolderId, volumeId int64, folderType int) *Dir {
	return NewDir(&File{
		Id:       virtualFolderId,
		ParentId: unKnown, //没有parent
		VolumeId: volumeId,
		Type:     folderType,
	}, 0, unKnown, unKnown)
}

func (d *Dir) IsVirtualDir() bool {
	return d.originInfo.Id <= 0
}

//FillDirNoRecurse 手动填充当前dir信息,不递归
func (d *Dir) FillDirNoRecurse(ctx context.Context, subFiles, subFolders []*File) error {
	if d.loaded {
		return errDirAlreadyLoaded
	}
	// 初始化
	d.size = 0 //初始化size
	d.subFiles = nil
	d.subDirs = nil
	d.count = int64(len(subFiles) + len(subFolders))
	for _, file := range subFiles {
		d.subFiles = append(d.subFiles, file)
		d.size += file.Size
	}
	for _, folder := range subFolders {
		tmpDir := NewDir(folder, d.depth+1, unKnown, unKnown) //把folder转成dir
		d.subDirs = append(d.subDirs, tmpDir)
	}
	d.loaded = true
	return nil
}

func (d *Dir) GetDirOriginInfo() *File {
	return d.originInfo
}

func (d *Dir) GetId() int64 {
	return d.originInfo.Id
}

func (d *Dir) GetSubDirs() []*Dir {
	return d.subDirs
}

//GetSubFiles 获取子文件,不递归
func (d *Dir) GetSubFiles() []*File {
	return d.subFiles
}

//GetSubFolders 获取子文件夹,不递归
func (d *Dir) GetSubFolders() []*File {
	var folders []*File
	for _, subDir := range d.subDirs {
		folders = append(folders, subDir.originInfo)
	}
	return folders
}

//GetSubFoldersAndFiles 获取子文件夹和子文件,不递归
func (d *Dir) GetSubFoldersAndFiles() []*File {
	var allFiles []*File
	for _, subDir := range d.subDirs {
		allFiles = append(allFiles, subDir.originInfo)
	}
	allFiles = append(allFiles, d.subFiles...)
	return allFiles
}

//GetAllPureFiles 获取所有纯文件
func (d *Dir) GetAllPureFiles(ctx context.Context) []*File {
	var allFiles []*File
	addSubFiles := func(ctx context.Context, dir *Dir) error {
		allFiles = append(allFiles, dir.subFiles...)
		return nil
	}
	_ = d.DfsWithFunc(ctx, addSubFiles, nil)
	return allFiles
}

//GetAllFolders 注意:不包括虚节点,DFS顺序返回的
func (d *Dir) GetAllFolders(ctx context.Context) []*File {
	var allFiles []*File

	if !d.IsVirtualDir() { //非虚拟的才包括根文件夹
		allFiles = append(allFiles, d.originInfo)
	}

	addSubFiles := func(ctx context.Context, dir *Dir) error {
		allFiles = append(allFiles, dir.GetSubFolders()...)
		return nil
	}
	_ = d.DfsWithFunc(ctx, addSubFiles, nil)
	return allFiles
}

//GetAllFoldersAndFiles 注意:不包括虚节点,dir类型DFS顺序返回的,可参考file_dir_test.go
func (d *Dir) GetAllFoldersAndFiles(ctx context.Context) []*File {
	var allFiles []*File
	if !d.IsVirtualDir() { //非虚拟的才包括根文件夹
		allFiles = append(allFiles, d.originInfo)
	}
	addSubFoldersAndFiles := func(ctx context.Context, dir *Dir) error {
		allFiles = append(allFiles, dir.GetSubFoldersAndFiles()...)
		return nil
	}
	_ = d.DfsWithFunc(ctx, addSubFoldersAndFiles, nil)
	return allFiles
}

//GetAllFoldersAndFilesByBfs 注意:不包括虚节点,BFS顺序返回
func (d *Dir) GetAllFoldersAndFilesByBfs(ctx context.Context) []*File {
	var allFiles []*File
	if !d.IsVirtualDir() { //非虚拟的才包括根文件夹
		allFiles = append(allFiles, d.originInfo)
	}
	addSubFoldersAndFiles := func(ctx context.Context, dir *Dir) error {
		allFiles = append(allFiles, dir.GetSubFoldersAndFiles()...)
		return nil
	}
	_ = d.BfsWithFunc(ctx, addSubFoldersAndFiles)
	return allFiles
}

//GetAllFoldersAndFilesOnLevel 层级维度的返回 注意:不包括虚节点,BFS顺序返回,而且是带层级的
func (d *Dir) GetAllFoldersAndFilesOnLevel(ctx context.Context) (levelAllFiles [][]*File) {
	var minLevel int64 //最小层级
	levelFilesMap := make(map[int64][]*File)
	if !d.IsVirtualDir() { //非虚拟的才包括根文件夹,第一层
		minLevel = d.depth
		levelFilesMap[d.depth] = []*File{d.originInfo}
	} else {
		minLevel = d.depth + 1
	}

	maxLevel := minLevel

	addSubFoldersAndFiles := func(ctx context.Context, dir *Dir) error {
		nextLevel := dir.depth + 1
		if nextLevel > maxLevel {
			maxLevel = nextLevel
		}
		levelFilesMap[nextLevel] = append(levelFilesMap[nextLevel], dir.GetSubFoldersAndFiles()...)
		return nil
	}
	_ = d.BfsWithFunc(ctx, addSubFoldersAndFiles)
	for level := minLevel; level <= maxLevel; level++ {
		levelAllFiles = append(levelAllFiles, levelFilesMap[level])
	}
	return levelAllFiles
}

func (d *Dir) GetDepth() int64 {
	return d.depth
}

func (d *Dir) GetCount() int64 {
	return d.count
}

func (d *Dir) GetSize() int64 {
	return d.size
}

func (d *Dir) IsLoaded() bool {
	return d.loaded
}

// dfs加载过程中的信息
type dsfLoadInfo struct {
	maxDepth   int64
	sizeLimit  int64
	numLimit   int64
	totalCount int64
	totalSize  int64
}

//FileNumLimit
func (d *Dir) DFSLoad(ctx context.Context,
	maxDepth, numLimit, sizeLimit int64,
	retrieveNextDepthFiles RetrieveNextDepthFilesFunc,
	preorderFunc, postorderFunc DirFunc) (totalSize, totalCount int64, err error) {
	if maxDepth < 0 {
		maxDepth = defMaxDepth
	}
	if numLimit < 0 {
		numLimit = defMaxTotalCount
	}
	dfsInfo := &dsfLoadInfo{
		maxDepth:   maxDepth,
		sizeLimit:  sizeLimit,
		numLimit:   numLimit,
		totalCount: 0,
		totalSize:  0,
	}
	if !d.IsVirtualDir() {
		dfsInfo.totalCount += 1 //非虚拟目录,算上根节点
	}
	err = d.dfsLoadDir(ctx, dfsInfo, retrieveNextDepthFiles, preorderFunc, postorderFunc)
	if err != nil {
		return 0, 0, err
	}
	return dfsInfo.totalSize, dfsInfo.totalCount, nil
}

func (d *Dir) DoDfsPreorderFunc(ctx context.Context, preorderFunc DirFunc) (err error) {
	return d.DfsWithFunc(ctx, preorderFunc, nil)
}

func (d *Dir) DoDfsPostorderFunc(ctx context.Context, postorderFunc DirFunc) (err error) {
	return d.DfsWithFunc(ctx, nil, postorderFunc)
}

func (d *Dir) GetTotalSizeAndCount(ctx context.Context) (totalSize, totalCount int64, err error) {
	if !d.IsVirtualDir() {
		totalCount += 1 //非虚拟目录,算上根节点
	}
	addSizeAndCount := func(ctx context.Context, dir *Dir) error {
		totalSize += dir.size
		totalCount += dir.count
		return nil
	}
	err = d.DfsWithFunc(ctx, addSizeAndCount, nil)
	if err != nil {
		return 0, 0, err
	}
	return
}

// load
func (d *Dir) dfsLoadDir(ctx context.Context,
	dfsInfo *dsfLoadInfo, retrieveNextDepthFiles RetrieveNextDepthFilesFunc,
	preorderFunc, postorderFunc DirFunc) error {
	if !d.originInfo.IsFolder() {
		return errNotFolderType
	}

	if d.depth >= dfsInfo.maxDepth {
		fmt.Printf("dfsLoadDir max recursion depth touch,currDepth=%d,maxDepth=%d", d.depth, dfsInfo.maxDepth)
		return errMaxPathDepthLimit
	}

	if preorderFunc != nil {
		if err := preorderFunc(ctx, d); err != nil {
			return err
		}
	}

	if !d.loaded {
		if retrieveNextDepthFiles == nil {
			return errNoRetrieveNextDepthFilesFunc
		}
		files, folders, err := retrieveNextDepthFiles(ctx, d.originInfo.VolumeId, d.originInfo.Id)
		if err != nil {
			return err
		}
		err = d.FillDirNoRecurse(ctx, files, folders)
		if err != nil {
			return err
		}
	}

	dfsInfo.totalCount += d.count
	dfsInfo.totalSize += d.size

	if dfsInfo.totalCount > dfsInfo.numLimit {
		fmt.Printf("dfsLoadDir totalCount=%d,fileNumLimit=%d,", dfsInfo.totalCount, dfsInfo.numLimit)
		return errFileNumLimit
	}

	if dfsInfo.sizeLimit >= 0 && dfsInfo.totalSize > dfsInfo.sizeLimit {
		fmt.Printf("dfsLoadDir totalCount=%d,totalSizeLimit=%d,", dfsInfo.totalCount, dfsInfo.sizeLimit)
		return errTotalSizeLimit
	}

	for _, subDir := range d.subDirs {
		if err := subDir.dfsLoadDir(ctx, dfsInfo, retrieveNextDepthFiles, preorderFunc, postorderFunc); err != nil {
			return err
		}
	}

	if postorderFunc != nil {
		if err := postorderFunc(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

//DfsWithFunc dfs遍历(针对dir节点),调用时需要已经load数据
func (d *Dir) DfsWithFunc(ctx context.Context, preorderFunc, postorderFunc DirFunc) (err error) {
	if !d.loaded {
		return errDirNotLoad
	}
	if preorderFunc != nil {
		if err = preorderFunc(ctx, d); err != nil {
			return err
		}
	}

	for _, dir := range d.subDirs {
		if err = dir.DfsWithFunc(ctx, preorderFunc, postorderFunc); err != nil {
			return err
		}
	}

	if postorderFunc != nil {
		if err = postorderFunc(ctx, d); err != nil {
			return err
		}
	}

	return nil
}

//BfsWithFunc Bfs遍历,注意:调用时需要已经load数据
func (d *Dir) BfsWithFunc(ctx context.Context, callBack DirFunc) (err error) {
	if !d.loaded {
		return errDirNotLoad
	}
	currDepthDirs := []*Dir{d} //当前层Dir

	for {
		if len(currDepthDirs) == 0 {
			break
		}
		var nextDepthDirs []*Dir //下一层Dir
		for _, dir := range currDepthDirs {
			if callBack != nil {
				err = callBack(ctx, dir)
				if err != nil {
					return err
				}
			}
			nextDepthDirs = append(nextDepthDirs, dir.subDirs...)
		}
		currDepthDirs = nextDepthDirs
	}

	return nil
}
