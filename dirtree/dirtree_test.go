package dirtree

import (
	"context"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// 这里使用父子层级模拟树
var fakeParentAndSons = map[*File][]*File{}

// root在数据库中是不存在的
func initRootFakeParent(volumeId, rootId int64) {
	rootFile := &File{
		Id:       rootId,
		VolumeId: volumeId,
		Type:     typeFolder,
	}
	fakeParentAndSons[rootFile] = nil
}

// 测试相关的func
func getSubFilesMock(ctx context.Context, volumeId, parentId int64) (files, folders []*File, err error) {
	parentFile := findParentNodeById(parentId)
	if parentFile == nil {
		return
	}
	subAllFiles := fakeParentAndSons[parentFile]
	files, folders = filterFilesAndFolders(subAllFiles)
	return
}

// filterFilesAndFolders
func filterFilesAndFolders(allFiles []*File) (files []*File, folders []*File) {
	for _, file := range allFiles {
		if file.IsFile() {
			files = append(files, file)
		}
		if file.IsFolder() {
			folders = append(folders, file)
		}
	}
	return
}

func insertFakeFile(volumeId, parentId, fileId, fileSize int64, fileType int) {
	name := fmt.Sprintf("%v-%v", parentId, fileId)
	if fileType == typeFolder {
		fileSize = 0
	}
	newFile := &File{
		Id:       fileId,
		ParentId: parentId,
		VolumeId: volumeId,
		Name:     name,
		Type:     fileType,
		Version:  1,
		Size:     fileSize,
	}
	parentFile := findParentNodeById(parentId)
	if parentFile == nil {
		panic(fmt.Sprintf("parent=%v not exsit", parentId))
	}
	fakeParentAndSons[parentFile] = append(fakeParentAndSons[parentFile], newFile)
	if newFile.Type == typeFolder {
		fakeParentAndSons[newFile] = nil
	}
}
func findParentNodeById(parentId int64) *File {
	for parentNode := range fakeParentAndSons {
		if parentNode.Id == parentId {
			return parentNode
		}
	}
	return nil
}

func buildTreeForTest() {
	/*
	  0
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
	*/

	fakeParentAndSons = make(map[*File][]*File)
	volumeId := int64(1)
	// ++++++++++++根目录+++++++++++++
	initRootFakeParent(volumeId, 0)
	// ++++++++++++++第一层++++++++++++++
	insertFakeFile(volumeId, 0, 10, 1, typeFile)
	insertFakeFile(volumeId, 0, 11, 1, typeFile)
	insertFakeFile(volumeId, 0, 12, 0, typeFolder)
	insertFakeFile(volumeId, 0, 13, 0, typeFolder)
	// ++++++++++++++第二层++++++++++++++
	insertFakeFile(volumeId, 12, 20, 1, typeFile)
	insertFakeFile(volumeId, 12, 21, 1, typeFile)
	insertFakeFile(volumeId, 12, 22, 0, typeFolder)
	insertFakeFile(volumeId, 12, 23, 0, typeFolder)

	insertFakeFile(volumeId, 13, 24, 0, typeFolder)
	// ++++++++++++++第三层++++++++++++++
	insertFakeFile(volumeId, 22, 30, 1, typeFile)
	insertFakeFile(volumeId, 22, 31, 1, typeFile)
	insertFakeFile(volumeId, 22, 32, 1, typeFile)
	insertFakeFile(volumeId, 22, 33, 0, typeFolder)

	insertFakeFile(volumeId, 23, 34, 1, typeFile)
	insertFakeFile(volumeId, 23, 35, 0, typeFolder)
	insertFakeFile(volumeId, 23, 36, 0, typeFolder)

	insertFakeFile(volumeId, 24, 37, 0, typeFolder)
	// ++++++++++++++第四层++++++++++++++
	insertFakeFile(volumeId, 33, 41, 1, typeFile)

	insertFakeFile(volumeId, 37, 42, 1, typeFile)
}

func newNewVirtualDirForTest() *Dir {
	volumeId := int64(1)
	return NewVirtualDir(0, volumeId, typeFolder)
}
func TestFileDirLoad(t *testing.T) {
	Convey("TestFileDirLoad", t, func() {
		Convey("TestFileDirLoad success", func() {
			buildTreeForTest()
			dir := newNewVirtualDirForTest()
			totalSize, totalCount, err := dir.DFSLoad(nil, -1, -1, -1, getSubFilesMock, nil, nil)
			So(err, ShouldBeNil)
			So(totalCount, ShouldEqual, 19)
			So(totalSize, ShouldEqual, 10)
		})

		Convey("TestFileDirLoad WithPostorderFunc calculateSize", func() {
			buildTreeForTest()
			dir := newNewVirtualDirForTest()
			var totalSize, totalCount int64 = 0, 0
			addSizeAndCount := func(ctx context.Context, dir *Dir) error {
				totalSize += dir.size
				totalCount += dir.count
				return nil
			}
			totalSize2, totalCount2, err := dir.DFSLoad(nil, -1, -1, -1, getSubFilesMock, nil, addSizeAndCount)
			So(err, ShouldBeNil)
			So(totalCount, ShouldEqual, 19)
			So(totalSize, ShouldEqual, 10)
			So(totalCount, ShouldEqual, totalCount2)
			So(totalSize, ShouldEqual, totalSize2)
		})

		Convey("TestFileDirLoad ErrDirMaxDepthLimit", func() {
			buildTreeForTest()
			dir := newNewVirtualDirForTest()
			_, _, err := dir.DFSLoad(nil, 2, -1, -1, getSubFilesMock, nil, nil)
			So(err, ShouldEqual, errMaxPathDepthLimit)
		})

		Convey("TestFileDirLoad ErrFileNumLimit", func() {
			buildTreeForTest()
			dir := newNewVirtualDirForTest()
			_, _, err := dir.DFSLoad(nil, -1, 18, -1, getSubFilesMock, nil, nil)
			So(err, ShouldEqual, errFileNumLimit)
		})

	})
}

func TestGetTotalSizeAndCount(t *testing.T) {
	buildTreeForTest()
	dir := newNewVirtualDirForTest()
	_, _, err := dir.DFSLoad(nil, -1, -1, -1, getSubFilesMock, nil, nil)
	if err != nil {
		fmt.Printf("dfsLoadDir err=%v", err)
	}
	Convey("TestGetTotalSizeAndCount", t, func() {
		totalSize, totalCount, err := dir.GetTotalSizeAndCount(nil)
		So(err, ShouldBeNil)
		So(totalCount, ShouldEqual, 19)
		So(totalSize, ShouldEqual, 10)
	})
}

func ExampleGetFoldersAndFiles() {
	buildTreeForTest()
	dir := newNewVirtualDirForTest()
	_, _, err := dir.DFSLoad(nil, -1, -1, -1, getSubFilesMock, nil, nil)
	if err != nil {
		fmt.Printf("dfsLoadDir err=%v", err)
	}
	fmt.Println("--------GetAllPureFiles---------")
	allPureFiles := dir.GetAllPureFiles(nil)
	for _, file := range allPureFiles {
		fmt.Printf("id=%d,parent=%d,name=%s,type=%s\n", file.Id, file.ParentId, file.Name, file.TypeString())
	}
	fmt.Println("--------GetAllFolders---------")
	allFolders := dir.GetAllFolders(nil)
	for _, file := range allFolders {
		fmt.Printf("id=%d,parent=%d,name=%s,type=%s\n", file.Id, file.ParentId, file.Name, file.TypeString())
	}

	fmt.Println("--------GetAllFoldersAndFiles---------")
	allFiles := dir.GetAllFoldersAndFiles(nil)
	for _, file := range allFiles {
		fmt.Printf("id=%d,parent=%d,name=%s,type=%s\n", file.Id, file.ParentId, file.Name, file.TypeString())
	}

	fmt.Println("--------GetAllFoldersAndFilesByBfs---------")
	allFiles = dir.GetAllFoldersAndFilesByBfs(nil)
	for _, file := range allFiles {
		fmt.Printf("id=%d,parent=%d,name=%s,type=%s\n", file.Id, file.ParentId, file.Name, file.TypeString())
	}

	fmt.Println("--------GetAllFoldersAndFilesOnLevel---------")
	levelAllFiles := dir.GetAllFoldersAndFilesOnLevel(nil)
	for i, levelFiles := range levelAllFiles {
		fmt.Printf("--------level:%v---------\n", i)
		for _, file := range levelFiles {
			fmt.Printf("id=%d,parent=%d,name=%s,type=%s\n", file.Id, file.ParentId, file.Name, file.TypeString())
		}
	}
	//Output:
	//--------GetAllPureFiles---------
	//id=10,parent=0,name=0-10,type=file
	//id=11,parent=0,name=0-11,type=file
	//id=20,parent=12,name=12-20,type=file
	//id=21,parent=12,name=12-21,type=file
	//id=30,parent=22,name=22-30,type=file
	//id=31,parent=22,name=22-31,type=file
	//id=32,parent=22,name=22-32,type=file
	//id=41,parent=33,name=33-41,type=file
	//id=34,parent=23,name=23-34,type=file
	//id=42,parent=37,name=37-42,type=file
	//--------GetAllFolders---------
	//id=12,parent=0,name=0-12,type=folder
	//id=13,parent=0,name=0-13,type=folder
	//id=22,parent=12,name=12-22,type=folder
	//id=23,parent=12,name=12-23,type=folder
	//id=33,parent=22,name=22-33,type=folder
	//id=35,parent=23,name=23-35,type=folder
	//id=36,parent=23,name=23-36,type=folder
	//id=24,parent=13,name=13-24,type=folder
	//id=37,parent=24,name=24-37,type=folder
	//--------GetAllFoldersAndFiles---------
	//id=12,parent=0,name=0-12,type=folder
	//id=13,parent=0,name=0-13,type=folder
	//id=10,parent=0,name=0-10,type=file
	//id=11,parent=0,name=0-11,type=file
	//id=22,parent=12,name=12-22,type=folder
	//id=23,parent=12,name=12-23,type=folder
	//id=20,parent=12,name=12-20,type=file
	//id=21,parent=12,name=12-21,type=file
	//id=33,parent=22,name=22-33,type=folder
	//id=30,parent=22,name=22-30,type=file
	//id=31,parent=22,name=22-31,type=file
	//id=32,parent=22,name=22-32,type=file
	//id=41,parent=33,name=33-41,type=file
	//id=35,parent=23,name=23-35,type=folder
	//id=36,parent=23,name=23-36,type=folder
	//id=34,parent=23,name=23-34,type=file
	//id=24,parent=13,name=13-24,type=folder
	//id=37,parent=24,name=24-37,type=folder
	//id=42,parent=37,name=37-42,type=file
	//--------GetAllFoldersAndFilesByBfs---------
	//id=12,parent=0,name=0-12,type=folder
	//id=13,parent=0,name=0-13,type=folder
	//id=10,parent=0,name=0-10,type=file
	//id=11,parent=0,name=0-11,type=file
	//id=22,parent=12,name=12-22,type=folder
	//id=23,parent=12,name=12-23,type=folder
	//id=20,parent=12,name=12-20,type=file
	//id=21,parent=12,name=12-21,type=file
	//id=24,parent=13,name=13-24,type=folder
	//id=33,parent=22,name=22-33,type=folder
	//id=30,parent=22,name=22-30,type=file
	//id=31,parent=22,name=22-31,type=file
	//id=32,parent=22,name=22-32,type=file
	//id=35,parent=23,name=23-35,type=folder
	//id=36,parent=23,name=23-36,type=folder
	//id=34,parent=23,name=23-34,type=file
	//id=37,parent=24,name=24-37,type=folder
	//id=41,parent=33,name=33-41,type=file
	//id=42,parent=37,name=37-42,type=file
	//--------GetAllFoldersAndFilesOnLevel---------
	//--------level:0---------
	//id=12,parent=0,name=0-12,type=folder
	//id=13,parent=0,name=0-13,type=folder
	//id=10,parent=0,name=0-10,type=file
	//id=11,parent=0,name=0-11,type=file
	//--------level:1---------
	//id=22,parent=12,name=12-22,type=folder
	//id=23,parent=12,name=12-23,type=folder
	//id=20,parent=12,name=12-20,type=file
	//id=21,parent=12,name=12-21,type=file
	//id=24,parent=13,name=13-24,type=folder
	//--------level:2---------
	//id=33,parent=22,name=22-33,type=folder
	//id=30,parent=22,name=22-30,type=file
	//id=31,parent=22,name=22-31,type=file
	//id=32,parent=22,name=22-32,type=file
	//id=35,parent=23,name=23-35,type=folder
	//id=36,parent=23,name=23-36,type=folder
	//id=34,parent=23,name=23-34,type=file
	//id=37,parent=24,name=24-37,type=folder
	//--------level:3---------
	//id=41,parent=33,name=33-41,type=file
	//id=42,parent=37,name=37-42,type=file
}

// 遍历 "测试使用DoPreorderFunc和DoPostorderFunc遍历树"
func ExampleTraverse() {
	buildTreeForTest()
	dir := newNewVirtualDirForTest()
	_, _, err := dir.DFSLoad(nil, 10, -1, -1, getSubFilesMock, nil, nil)
	if err != nil {
		fmt.Printf("dfsLoadDir err=%v", err)
	}
	printDir := func(ctx context.Context, dir *Dir) error {
		fmt.Printf("----dir name=%v,depth=%v,size=%v,count=%v,loaded=%v\n", dir.originInfo.Name, dir.depth, dir.size, dir.count, dir.loaded)
		for _, folder := range dir.subDirs {
			fmt.Println("folder----", folder.originInfo.Name)
		}
		for _, file := range dir.subFiles {
			fmt.Println("file----", file.Name)
		}
		return nil
	}
	fmt.Println("+++++++++DoDfsPreorderFunc++++++++")
	if err := dir.DoDfsPreorderFunc(nil, printDir); err != nil {
		panic(fmt.Sprintf("DoDfsPreorderFunc err=%v", err))
	}
	fmt.Println("+++++++++DoDfsPostorderFunc++++++++")
	if err := dir.DoDfsPostorderFunc(nil, printDir); err != nil {
		panic(fmt.Sprintf("DoDfsPostorderFunc err=%v", err))
	}
	// Output:
	//+++++++++DoDfsPreorderFunc++++++++
	//----dir name=,depth=0,size=2,count=4,loaded=true
	//folder---- 0-12
	//folder---- 0-13
	//file---- 0-10
	//file---- 0-11
	//----dir name=0-12,depth=1,size=2,count=4,loaded=true
	//folder---- 12-22
	//folder---- 12-23
	//file---- 12-20
	//file---- 12-21
	//----dir name=12-22,depth=2,size=3,count=4,loaded=true
	//folder---- 22-33
	//file---- 22-30
	//file---- 22-31
	//file---- 22-32
	//----dir name=22-33,depth=3,size=1,count=1,loaded=true
	//file---- 33-41
	//----dir name=12-23,depth=2,size=1,count=3,loaded=true
	//folder---- 23-35
	//folder---- 23-36
	//file---- 23-34
	//----dir name=23-35,depth=3,size=0,count=0,loaded=true
	//----dir name=23-36,depth=3,size=0,count=0,loaded=true
	//----dir name=0-13,depth=1,size=0,count=1,loaded=true
	//folder---- 13-24
	//----dir name=13-24,depth=2,size=0,count=1,loaded=true
	//folder---- 24-37
	//----dir name=24-37,depth=3,size=1,count=1,loaded=true
	//file---- 37-42
	//+++++++++DoDfsPostorderFunc++++++++
	//----dir name=22-33,depth=3,size=1,count=1,loaded=true
	//file---- 33-41
	//----dir name=12-22,depth=2,size=3,count=4,loaded=true
	//folder---- 22-33
	//file---- 22-30
	//file---- 22-31
	//file---- 22-32
	//----dir name=23-35,depth=3,size=0,count=0,loaded=true
	//----dir name=23-36,depth=3,size=0,count=0,loaded=true
	//----dir name=12-23,depth=2,size=1,count=3,loaded=true
	//folder---- 23-35
	//folder---- 23-36
	//file---- 23-34
	//----dir name=0-12,depth=1,size=2,count=4,loaded=true
	//folder---- 12-22
	//folder---- 12-23
	//file---- 12-20
	//file---- 12-21
	//----dir name=24-37,depth=3,size=1,count=1,loaded=true
	//file---- 37-42
	//----dir name=13-24,depth=2,size=0,count=1,loaded=true
	//folder---- 24-37
	//----dir name=0-13,depth=1,size=0,count=1,loaded=true
	//folder---- 13-24
	//----dir name=,depth=0,size=2,count=4,loaded=true
	//folder---- 0-12
	//folder---- 0-13
	//file---- 0-10
	//file---- 0-11
}

// 遍历 "测试手动先增加一层目录信息"
func ExampleManualLoadTraverse() {
	buildTreeForTest()
	dir := newNewVirtualDirForTest()
	files, folders, _ := getSubFilesMock(nil, 1, 0)
	dir.FillDirNoRecurse(nil, files, folders) //手动增加一层

	_, _, err := dir.DFSLoad(nil, 10, -1, -1, getSubFilesMock, nil, nil)
	if err != nil {
		fmt.Printf("dfsLoadDir err=%v", err)
	}
	printDir := func(ctx context.Context, dir *Dir) error {
		fmt.Printf("----dir name=%v,depth=%v,size=%v,count=%v,loaded=%v\n", dir.originInfo.Name, dir.depth, dir.size, dir.count, dir.loaded)
		for _, folder := range dir.subDirs {
			fmt.Println("folder----", folder.originInfo.Name)
		}
		for _, file := range dir.subFiles {
			fmt.Println("file----", file.Name)
		}
		return nil
	}
	fmt.Println("+++++++++DoDfsPreorderFunc++++++++")
	if err := dir.DoDfsPreorderFunc(nil, printDir); err != nil {
		panic(fmt.Sprintf("DoDfsPreorderFunc err=%v", err))
	}
	fmt.Println("+++++++++DoDfsPostorderFunc++++++++")
	if err := dir.DoDfsPostorderFunc(nil, printDir); err != nil {
		panic(fmt.Sprintf("DoDfsPostorderFunc err=%v", err))
	}
	// Output:
	//+++++++++DoDfsPreorderFunc++++++++
	//----dir name=,depth=0,size=2,count=4,loaded=true
	//folder---- 0-12
	//folder---- 0-13
	//file---- 0-10
	//file---- 0-11
	//----dir name=0-12,depth=1,size=2,count=4,loaded=true
	//folder---- 12-22
	//folder---- 12-23
	//file---- 12-20
	//file---- 12-21
	//----dir name=12-22,depth=2,size=3,count=4,loaded=true
	//folder---- 22-33
	//file---- 22-30
	//file---- 22-31
	//file---- 22-32
	//----dir name=22-33,depth=3,size=1,count=1,loaded=true
	//file---- 33-41
	//----dir name=12-23,depth=2,size=1,count=3,loaded=true
	//folder---- 23-35
	//folder---- 23-36
	//file---- 23-34
	//----dir name=23-35,depth=3,size=0,count=0,loaded=true
	//----dir name=23-36,depth=3,size=0,count=0,loaded=true
	//----dir name=0-13,depth=1,size=0,count=1,loaded=true
	//folder---- 13-24
	//----dir name=13-24,depth=2,size=0,count=1,loaded=true
	//folder---- 24-37
	//----dir name=24-37,depth=3,size=1,count=1,loaded=true
	//file---- 37-42
	//+++++++++DoDfsPostorderFunc++++++++
	//----dir name=22-33,depth=3,size=1,count=1,loaded=true
	//file---- 33-41
	//----dir name=12-22,depth=2,size=3,count=4,loaded=true
	//folder---- 22-33
	//file---- 22-30
	//file---- 22-31
	//file---- 22-32
	//----dir name=23-35,depth=3,size=0,count=0,loaded=true
	//----dir name=23-36,depth=3,size=0,count=0,loaded=true
	//----dir name=12-23,depth=2,size=1,count=3,loaded=true
	//folder---- 23-35
	//folder---- 23-36
	//file---- 23-34
	//----dir name=0-12,depth=1,size=2,count=4,loaded=true
	//folder---- 12-22
	//folder---- 12-23
	//file---- 12-20
	//file---- 12-21
	//----dir name=24-37,depth=3,size=1,count=1,loaded=true
	//file---- 37-42
	//----dir name=13-24,depth=2,size=0,count=1,loaded=true
	//folder---- 24-37
	//----dir name=0-13,depth=1,size=0,count=1,loaded=true
	//folder---- 13-24
	//----dir name=,depth=0,size=2,count=4,loaded=true
	//folder---- 0-12
	//folder---- 0-13
	//file---- 0-10
	//file---- 0-11
}
