// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/cloud/automl/v1beta1/data_stats.proto

package automl

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// The data statistics of a series of values that share the same DataType.
type DataStats struct {
	// The data statistics specific to a DataType.
	//
	// Types that are valid to be assigned to Stats:
	//	*DataStats_Float64Stats
	//	*DataStats_StringStats
	//	*DataStats_TimestampStats
	//	*DataStats_ArrayStats
	//	*DataStats_StructStats
	//	*DataStats_CategoryStats
	Stats isDataStats_Stats `protobuf_oneof:"stats"`
	// The number of distinct values.
	DistinctValueCount int64 `protobuf:"varint,1,opt,name=distinct_value_count,json=distinctValueCount,proto3" json:"distinct_value_count,omitempty"`
	// The number of values that are null.
	NullValueCount int64 `protobuf:"varint,2,opt,name=null_value_count,json=nullValueCount,proto3" json:"null_value_count,omitempty"`
	// The number of values that are valid.
	ValidValueCount      int64    `protobuf:"varint,9,opt,name=valid_value_count,json=validValueCount,proto3" json:"valid_value_count,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DataStats) Reset()         { *m = DataStats{} }
func (m *DataStats) String() string { return proto.CompactTextString(m) }
func (*DataStats) ProtoMessage()    {}
func (*DataStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{0}
}

func (m *DataStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DataStats.Unmarshal(m, b)
}
func (m *DataStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DataStats.Marshal(b, m, deterministic)
}
func (m *DataStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DataStats.Merge(m, src)
}
func (m *DataStats) XXX_Size() int {
	return xxx_messageInfo_DataStats.Size(m)
}
func (m *DataStats) XXX_DiscardUnknown() {
	xxx_messageInfo_DataStats.DiscardUnknown(m)
}

var xxx_messageInfo_DataStats proto.InternalMessageInfo

type isDataStats_Stats interface {
	isDataStats_Stats()
}

type DataStats_Float64Stats struct {
	Float64Stats *Float64Stats `protobuf:"bytes,3,opt,name=float64_stats,json=float64Stats,proto3,oneof"`
}

type DataStats_StringStats struct {
	StringStats *StringStats `protobuf:"bytes,4,opt,name=string_stats,json=stringStats,proto3,oneof"`
}

type DataStats_TimestampStats struct {
	TimestampStats *TimestampStats `protobuf:"bytes,5,opt,name=timestamp_stats,json=timestampStats,proto3,oneof"`
}

type DataStats_ArrayStats struct {
	ArrayStats *ArrayStats `protobuf:"bytes,6,opt,name=array_stats,json=arrayStats,proto3,oneof"`
}

type DataStats_StructStats struct {
	StructStats *StructStats `protobuf:"bytes,7,opt,name=struct_stats,json=structStats,proto3,oneof"`
}

type DataStats_CategoryStats struct {
	CategoryStats *CategoryStats `protobuf:"bytes,8,opt,name=category_stats,json=categoryStats,proto3,oneof"`
}

func (*DataStats_Float64Stats) isDataStats_Stats() {}

func (*DataStats_StringStats) isDataStats_Stats() {}

func (*DataStats_TimestampStats) isDataStats_Stats() {}

func (*DataStats_ArrayStats) isDataStats_Stats() {}

func (*DataStats_StructStats) isDataStats_Stats() {}

func (*DataStats_CategoryStats) isDataStats_Stats() {}

func (m *DataStats) GetStats() isDataStats_Stats {
	if m != nil {
		return m.Stats
	}
	return nil
}

func (m *DataStats) GetFloat64Stats() *Float64Stats {
	if x, ok := m.GetStats().(*DataStats_Float64Stats); ok {
		return x.Float64Stats
	}
	return nil
}

func (m *DataStats) GetStringStats() *StringStats {
	if x, ok := m.GetStats().(*DataStats_StringStats); ok {
		return x.StringStats
	}
	return nil
}

func (m *DataStats) GetTimestampStats() *TimestampStats {
	if x, ok := m.GetStats().(*DataStats_TimestampStats); ok {
		return x.TimestampStats
	}
	return nil
}

func (m *DataStats) GetArrayStats() *ArrayStats {
	if x, ok := m.GetStats().(*DataStats_ArrayStats); ok {
		return x.ArrayStats
	}
	return nil
}

func (m *DataStats) GetStructStats() *StructStats {
	if x, ok := m.GetStats().(*DataStats_StructStats); ok {
		return x.StructStats
	}
	return nil
}

func (m *DataStats) GetCategoryStats() *CategoryStats {
	if x, ok := m.GetStats().(*DataStats_CategoryStats); ok {
		return x.CategoryStats
	}
	return nil
}

func (m *DataStats) GetDistinctValueCount() int64 {
	if m != nil {
		return m.DistinctValueCount
	}
	return 0
}

func (m *DataStats) GetNullValueCount() int64 {
	if m != nil {
		return m.NullValueCount
	}
	return 0
}

func (m *DataStats) GetValidValueCount() int64 {
	if m != nil {
		return m.ValidValueCount
	}
	return 0
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*DataStats) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*DataStats_Float64Stats)(nil),
		(*DataStats_StringStats)(nil),
		(*DataStats_TimestampStats)(nil),
		(*DataStats_ArrayStats)(nil),
		(*DataStats_StructStats)(nil),
		(*DataStats_CategoryStats)(nil),
	}
}

// The data statistics of a series of FLOAT64 values.
type Float64Stats struct {
	// The mean of the series.
	Mean float64 `protobuf:"fixed64,1,opt,name=mean,proto3" json:"mean,omitempty"`
	// The standard deviation of the series.
	StandardDeviation float64 `protobuf:"fixed64,2,opt,name=standard_deviation,json=standardDeviation,proto3" json:"standard_deviation,omitempty"`
	// Ordered from 0 to k k-quantile values of the data series of n values.
	// The value at index i is, approximately, the i*n/k-th smallest value in the
	// series; for i = 0 and i = k these are, respectively, the min and max
	// values.
	Quantiles []float64 `protobuf:"fixed64,3,rep,packed,name=quantiles,proto3" json:"quantiles,omitempty"`
	// Histogram buckets of the data series. Sorted by the min value of the
	// bucket, ascendingly, and the number of the buckets is dynamically
	// generated. The buckets are non-overlapping and completely cover whole
	// FLOAT64 range with min of first bucket being `"-Infinity"`, and max of
	// the last one being `"Infinity"`.
	HistogramBuckets     []*Float64Stats_HistogramBucket `protobuf:"bytes,4,rep,name=histogram_buckets,json=histogramBuckets,proto3" json:"histogram_buckets,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                        `json:"-"`
	XXX_unrecognized     []byte                          `json:"-"`
	XXX_sizecache        int32                           `json:"-"`
}

func (m *Float64Stats) Reset()         { *m = Float64Stats{} }
func (m *Float64Stats) String() string { return proto.CompactTextString(m) }
func (*Float64Stats) ProtoMessage()    {}
func (*Float64Stats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{1}
}

func (m *Float64Stats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Float64Stats.Unmarshal(m, b)
}
func (m *Float64Stats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Float64Stats.Marshal(b, m, deterministic)
}
func (m *Float64Stats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Float64Stats.Merge(m, src)
}
func (m *Float64Stats) XXX_Size() int {
	return xxx_messageInfo_Float64Stats.Size(m)
}
func (m *Float64Stats) XXX_DiscardUnknown() {
	xxx_messageInfo_Float64Stats.DiscardUnknown(m)
}

var xxx_messageInfo_Float64Stats proto.InternalMessageInfo

func (m *Float64Stats) GetMean() float64 {
	if m != nil {
		return m.Mean
	}
	return 0
}

func (m *Float64Stats) GetStandardDeviation() float64 {
	if m != nil {
		return m.StandardDeviation
	}
	return 0
}

func (m *Float64Stats) GetQuantiles() []float64 {
	if m != nil {
		return m.Quantiles
	}
	return nil
}

func (m *Float64Stats) GetHistogramBuckets() []*Float64Stats_HistogramBucket {
	if m != nil {
		return m.HistogramBuckets
	}
	return nil
}

// A bucket of a histogram.
type Float64Stats_HistogramBucket struct {
	// The minimum value of the bucket, inclusive.
	Min float64 `protobuf:"fixed64,1,opt,name=min,proto3" json:"min,omitempty"`
	// The maximum value of the bucket, exclusive unless max = `"Infinity"`, in
	// which case it's inclusive.
	Max float64 `protobuf:"fixed64,2,opt,name=max,proto3" json:"max,omitempty"`
	// The number of data values that are in the bucket, i.e. are between
	// min and max values.
	Count                int64    `protobuf:"varint,3,opt,name=count,proto3" json:"count,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Float64Stats_HistogramBucket) Reset()         { *m = Float64Stats_HistogramBucket{} }
func (m *Float64Stats_HistogramBucket) String() string { return proto.CompactTextString(m) }
func (*Float64Stats_HistogramBucket) ProtoMessage()    {}
func (*Float64Stats_HistogramBucket) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{1, 0}
}

func (m *Float64Stats_HistogramBucket) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Float64Stats_HistogramBucket.Unmarshal(m, b)
}
func (m *Float64Stats_HistogramBucket) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Float64Stats_HistogramBucket.Marshal(b, m, deterministic)
}
func (m *Float64Stats_HistogramBucket) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Float64Stats_HistogramBucket.Merge(m, src)
}
func (m *Float64Stats_HistogramBucket) XXX_Size() int {
	return xxx_messageInfo_Float64Stats_HistogramBucket.Size(m)
}
func (m *Float64Stats_HistogramBucket) XXX_DiscardUnknown() {
	xxx_messageInfo_Float64Stats_HistogramBucket.DiscardUnknown(m)
}

var xxx_messageInfo_Float64Stats_HistogramBucket proto.InternalMessageInfo

func (m *Float64Stats_HistogramBucket) GetMin() float64 {
	if m != nil {
		return m.Min
	}
	return 0
}

func (m *Float64Stats_HistogramBucket) GetMax() float64 {
	if m != nil {
		return m.Max
	}
	return 0
}

func (m *Float64Stats_HistogramBucket) GetCount() int64 {
	if m != nil {
		return m.Count
	}
	return 0
}

// The data statistics of a series of STRING values.
type StringStats struct {
	// The statistics of the top 20 unigrams, ordered by
	// [count][google.cloud.automl.v1beta1.StringStats.UnigramStats.count].
	TopUnigramStats      []*StringStats_UnigramStats `protobuf:"bytes,1,rep,name=top_unigram_stats,json=topUnigramStats,proto3" json:"top_unigram_stats,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                    `json:"-"`
	XXX_unrecognized     []byte                      `json:"-"`
	XXX_sizecache        int32                       `json:"-"`
}

func (m *StringStats) Reset()         { *m = StringStats{} }
func (m *StringStats) String() string { return proto.CompactTextString(m) }
func (*StringStats) ProtoMessage()    {}
func (*StringStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{2}
}

func (m *StringStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StringStats.Unmarshal(m, b)
}
func (m *StringStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StringStats.Marshal(b, m, deterministic)
}
func (m *StringStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StringStats.Merge(m, src)
}
func (m *StringStats) XXX_Size() int {
	return xxx_messageInfo_StringStats.Size(m)
}
func (m *StringStats) XXX_DiscardUnknown() {
	xxx_messageInfo_StringStats.DiscardUnknown(m)
}

var xxx_messageInfo_StringStats proto.InternalMessageInfo

func (m *StringStats) GetTopUnigramStats() []*StringStats_UnigramStats {
	if m != nil {
		return m.TopUnigramStats
	}
	return nil
}

// The statistics of a unigram.
type StringStats_UnigramStats struct {
	// The unigram.
	Value string `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
	// The number of occurrences of this unigram in the series.
	Count                int64    `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StringStats_UnigramStats) Reset()         { *m = StringStats_UnigramStats{} }
func (m *StringStats_UnigramStats) String() string { return proto.CompactTextString(m) }
func (*StringStats_UnigramStats) ProtoMessage()    {}
func (*StringStats_UnigramStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{2, 0}
}

func (m *StringStats_UnigramStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StringStats_UnigramStats.Unmarshal(m, b)
}
func (m *StringStats_UnigramStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StringStats_UnigramStats.Marshal(b, m, deterministic)
}
func (m *StringStats_UnigramStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StringStats_UnigramStats.Merge(m, src)
}
func (m *StringStats_UnigramStats) XXX_Size() int {
	return xxx_messageInfo_StringStats_UnigramStats.Size(m)
}
func (m *StringStats_UnigramStats) XXX_DiscardUnknown() {
	xxx_messageInfo_StringStats_UnigramStats.DiscardUnknown(m)
}

var xxx_messageInfo_StringStats_UnigramStats proto.InternalMessageInfo

func (m *StringStats_UnigramStats) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

func (m *StringStats_UnigramStats) GetCount() int64 {
	if m != nil {
		return m.Count
	}
	return 0
}

// The data statistics of a series of TIMESTAMP values.
type TimestampStats struct {
	// The string key is the pre-defined granularity. Currently supported:
	// hour_of_day, day_of_week, month_of_year.
	// Granularities finer that the granularity of timestamp data are not
	// populated (e.g. if timestamps are at day granularity, then hour_of_day
	// is not populated).
	GranularStats        map[string]*TimestampStats_GranularStats `protobuf:"bytes,1,rep,name=granular_stats,json=granularStats,proto3" json:"granular_stats,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}                                 `json:"-"`
	XXX_unrecognized     []byte                                   `json:"-"`
	XXX_sizecache        int32                                    `json:"-"`
}

func (m *TimestampStats) Reset()         { *m = TimestampStats{} }
func (m *TimestampStats) String() string { return proto.CompactTextString(m) }
func (*TimestampStats) ProtoMessage()    {}
func (*TimestampStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{3}
}

func (m *TimestampStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TimestampStats.Unmarshal(m, b)
}
func (m *TimestampStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TimestampStats.Marshal(b, m, deterministic)
}
func (m *TimestampStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TimestampStats.Merge(m, src)
}
func (m *TimestampStats) XXX_Size() int {
	return xxx_messageInfo_TimestampStats.Size(m)
}
func (m *TimestampStats) XXX_DiscardUnknown() {
	xxx_messageInfo_TimestampStats.DiscardUnknown(m)
}

var xxx_messageInfo_TimestampStats proto.InternalMessageInfo

func (m *TimestampStats) GetGranularStats() map[string]*TimestampStats_GranularStats {
	if m != nil {
		return m.GranularStats
	}
	return nil
}

// Stats split by a defined in context granularity.
type TimestampStats_GranularStats struct {
	// A map from granularity key to example count for that key.
	// E.g. for hour_of_day `13` means 1pm, or for month_of_year `5` means May).
	Buckets              map[int32]int64 `protobuf:"bytes,1,rep,name=buckets,proto3" json:"buckets,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *TimestampStats_GranularStats) Reset()         { *m = TimestampStats_GranularStats{} }
func (m *TimestampStats_GranularStats) String() string { return proto.CompactTextString(m) }
func (*TimestampStats_GranularStats) ProtoMessage()    {}
func (*TimestampStats_GranularStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{3, 0}
}

func (m *TimestampStats_GranularStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TimestampStats_GranularStats.Unmarshal(m, b)
}
func (m *TimestampStats_GranularStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TimestampStats_GranularStats.Marshal(b, m, deterministic)
}
func (m *TimestampStats_GranularStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TimestampStats_GranularStats.Merge(m, src)
}
func (m *TimestampStats_GranularStats) XXX_Size() int {
	return xxx_messageInfo_TimestampStats_GranularStats.Size(m)
}
func (m *TimestampStats_GranularStats) XXX_DiscardUnknown() {
	xxx_messageInfo_TimestampStats_GranularStats.DiscardUnknown(m)
}

var xxx_messageInfo_TimestampStats_GranularStats proto.InternalMessageInfo

func (m *TimestampStats_GranularStats) GetBuckets() map[int32]int64 {
	if m != nil {
		return m.Buckets
	}
	return nil
}

// The data statistics of a series of ARRAY values.
type ArrayStats struct {
	// Stats of all the values of all arrays, as if they were a single long
	// series of data. The type depends on the element type of the array.
	MemberStats          *DataStats `protobuf:"bytes,2,opt,name=member_stats,json=memberStats,proto3" json:"member_stats,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *ArrayStats) Reset()         { *m = ArrayStats{} }
func (m *ArrayStats) String() string { return proto.CompactTextString(m) }
func (*ArrayStats) ProtoMessage()    {}
func (*ArrayStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{4}
}

func (m *ArrayStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ArrayStats.Unmarshal(m, b)
}
func (m *ArrayStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ArrayStats.Marshal(b, m, deterministic)
}
func (m *ArrayStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ArrayStats.Merge(m, src)
}
func (m *ArrayStats) XXX_Size() int {
	return xxx_messageInfo_ArrayStats.Size(m)
}
func (m *ArrayStats) XXX_DiscardUnknown() {
	xxx_messageInfo_ArrayStats.DiscardUnknown(m)
}

var xxx_messageInfo_ArrayStats proto.InternalMessageInfo

func (m *ArrayStats) GetMemberStats() *DataStats {
	if m != nil {
		return m.MemberStats
	}
	return nil
}

// The data statistics of a series of STRUCT values.
type StructStats struct {
	// Map from a field name of the struct to data stats aggregated over series
	// of all data in that field across all the structs.
	FieldStats           map[string]*DataStats `protobuf:"bytes,1,rep,name=field_stats,json=fieldStats,proto3" json:"field_stats,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *StructStats) Reset()         { *m = StructStats{} }
func (m *StructStats) String() string { return proto.CompactTextString(m) }
func (*StructStats) ProtoMessage()    {}
func (*StructStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{5}
}

func (m *StructStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StructStats.Unmarshal(m, b)
}
func (m *StructStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StructStats.Marshal(b, m, deterministic)
}
func (m *StructStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StructStats.Merge(m, src)
}
func (m *StructStats) XXX_Size() int {
	return xxx_messageInfo_StructStats.Size(m)
}
func (m *StructStats) XXX_DiscardUnknown() {
	xxx_messageInfo_StructStats.DiscardUnknown(m)
}

var xxx_messageInfo_StructStats proto.InternalMessageInfo

func (m *StructStats) GetFieldStats() map[string]*DataStats {
	if m != nil {
		return m.FieldStats
	}
	return nil
}

// The data statistics of a series of CATEGORY values.
type CategoryStats struct {
	// The statistics of the top 20 CATEGORY values, ordered by
	//
	// [count][google.cloud.automl.v1beta1.CategoryStats.SingleCategoryStats.count].
	TopCategoryStats     []*CategoryStats_SingleCategoryStats `protobuf:"bytes,1,rep,name=top_category_stats,json=topCategoryStats,proto3" json:"top_category_stats,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                             `json:"-"`
	XXX_unrecognized     []byte                               `json:"-"`
	XXX_sizecache        int32                                `json:"-"`
}

func (m *CategoryStats) Reset()         { *m = CategoryStats{} }
func (m *CategoryStats) String() string { return proto.CompactTextString(m) }
func (*CategoryStats) ProtoMessage()    {}
func (*CategoryStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{6}
}

func (m *CategoryStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CategoryStats.Unmarshal(m, b)
}
func (m *CategoryStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CategoryStats.Marshal(b, m, deterministic)
}
func (m *CategoryStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CategoryStats.Merge(m, src)
}
func (m *CategoryStats) XXX_Size() int {
	return xxx_messageInfo_CategoryStats.Size(m)
}
func (m *CategoryStats) XXX_DiscardUnknown() {
	xxx_messageInfo_CategoryStats.DiscardUnknown(m)
}

var xxx_messageInfo_CategoryStats proto.InternalMessageInfo

func (m *CategoryStats) GetTopCategoryStats() []*CategoryStats_SingleCategoryStats {
	if m != nil {
		return m.TopCategoryStats
	}
	return nil
}

// The statistics of a single CATEGORY value.
type CategoryStats_SingleCategoryStats struct {
	// The CATEGORY value.
	Value string `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
	// The number of occurrences of this value in the series.
	Count                int64    `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CategoryStats_SingleCategoryStats) Reset()         { *m = CategoryStats_SingleCategoryStats{} }
func (m *CategoryStats_SingleCategoryStats) String() string { return proto.CompactTextString(m) }
func (*CategoryStats_SingleCategoryStats) ProtoMessage()    {}
func (*CategoryStats_SingleCategoryStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{6, 0}
}

func (m *CategoryStats_SingleCategoryStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CategoryStats_SingleCategoryStats.Unmarshal(m, b)
}
func (m *CategoryStats_SingleCategoryStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CategoryStats_SingleCategoryStats.Marshal(b, m, deterministic)
}
func (m *CategoryStats_SingleCategoryStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CategoryStats_SingleCategoryStats.Merge(m, src)
}
func (m *CategoryStats_SingleCategoryStats) XXX_Size() int {
	return xxx_messageInfo_CategoryStats_SingleCategoryStats.Size(m)
}
func (m *CategoryStats_SingleCategoryStats) XXX_DiscardUnknown() {
	xxx_messageInfo_CategoryStats_SingleCategoryStats.DiscardUnknown(m)
}

var xxx_messageInfo_CategoryStats_SingleCategoryStats proto.InternalMessageInfo

func (m *CategoryStats_SingleCategoryStats) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

func (m *CategoryStats_SingleCategoryStats) GetCount() int64 {
	if m != nil {
		return m.Count
	}
	return 0
}

// A correlation statistics between two series of DataType values. The series
// may have differing DataType-s, but within a single series the DataType must
// be the same.
type CorrelationStats struct {
	// The correlation value using the Cramer's V measure.
	CramersV             float64  `protobuf:"fixed64,1,opt,name=cramers_v,json=cramersV,proto3" json:"cramers_v,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CorrelationStats) Reset()         { *m = CorrelationStats{} }
func (m *CorrelationStats) String() string { return proto.CompactTextString(m) }
func (*CorrelationStats) ProtoMessage()    {}
func (*CorrelationStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_1f99b1d575d82961, []int{7}
}

func (m *CorrelationStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CorrelationStats.Unmarshal(m, b)
}
func (m *CorrelationStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CorrelationStats.Marshal(b, m, deterministic)
}
func (m *CorrelationStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CorrelationStats.Merge(m, src)
}
func (m *CorrelationStats) XXX_Size() int {
	return xxx_messageInfo_CorrelationStats.Size(m)
}
func (m *CorrelationStats) XXX_DiscardUnknown() {
	xxx_messageInfo_CorrelationStats.DiscardUnknown(m)
}

var xxx_messageInfo_CorrelationStats proto.InternalMessageInfo

func (m *CorrelationStats) GetCramersV() float64 {
	if m != nil {
		return m.CramersV
	}
	return 0
}

func init() {
	proto.RegisterType((*DataStats)(nil), "google.cloud.automl.v1beta1.DataStats")
	proto.RegisterType((*Float64Stats)(nil), "google.cloud.automl.v1beta1.Float64Stats")
	proto.RegisterType((*Float64Stats_HistogramBucket)(nil), "google.cloud.automl.v1beta1.Float64Stats.HistogramBucket")
	proto.RegisterType((*StringStats)(nil), "google.cloud.automl.v1beta1.StringStats")
	proto.RegisterType((*StringStats_UnigramStats)(nil), "google.cloud.automl.v1beta1.StringStats.UnigramStats")
	proto.RegisterType((*TimestampStats)(nil), "google.cloud.automl.v1beta1.TimestampStats")
	proto.RegisterMapType((map[string]*TimestampStats_GranularStats)(nil), "google.cloud.automl.v1beta1.TimestampStats.GranularStatsEntry")
	proto.RegisterType((*TimestampStats_GranularStats)(nil), "google.cloud.automl.v1beta1.TimestampStats.GranularStats")
	proto.RegisterMapType((map[int32]int64)(nil), "google.cloud.automl.v1beta1.TimestampStats.GranularStats.BucketsEntry")
	proto.RegisterType((*ArrayStats)(nil), "google.cloud.automl.v1beta1.ArrayStats")
	proto.RegisterType((*StructStats)(nil), "google.cloud.automl.v1beta1.StructStats")
	proto.RegisterMapType((map[string]*DataStats)(nil), "google.cloud.automl.v1beta1.StructStats.FieldStatsEntry")
	proto.RegisterType((*CategoryStats)(nil), "google.cloud.automl.v1beta1.CategoryStats")
	proto.RegisterType((*CategoryStats_SingleCategoryStats)(nil), "google.cloud.automl.v1beta1.CategoryStats.SingleCategoryStats")
	proto.RegisterType((*CorrelationStats)(nil), "google.cloud.automl.v1beta1.CorrelationStats")
}

func init() {
	proto.RegisterFile("google/cloud/automl/v1beta1/data_stats.proto", fileDescriptor_1f99b1d575d82961)
}

var fileDescriptor_1f99b1d575d82961 = []byte{
	// 863 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x56, 0xdd, 0x8e, 0xdb, 0x44,
	0x14, 0xc6, 0x49, 0xd3, 0x6d, 0x4e, 0x7e, 0x77, 0xe8, 0x45, 0x95, 0xad, 0xa0, 0xca, 0x05, 0x84,
	0x02, 0x36, 0x2d, 0x3f, 0x2a, 0x06, 0x21, 0xed, 0x6e, 0xd9, 0x96, 0x9f, 0x8a, 0xca, 0x0b, 0x8b,
	0x40, 0x2b, 0x85, 0x59, 0x7b, 0xe2, 0x5a, 0x1d, 0xcf, 0x04, 0x7b, 0x1c, 0xb1, 0xe2, 0x9a, 0x37,
	0x29, 0x77, 0xf0, 0x0e, 0x5c, 0x73, 0xc3, 0x7b, 0xf0, 0x14, 0xc8, 0x67, 0xc6, 0xc9, 0x78, 0x59,
	0x99, 0x2c, 0x77, 0x3e, 0xe7, 0x7c, 0xe7, 0x3b, 0xbf, 0x73, 0x12, 0x78, 0x2b, 0x96, 0x32, 0xe6,
	0xcc, 0x0b, 0xb9, 0x2c, 0x22, 0x8f, 0x16, 0x4a, 0xa6, 0xdc, 0x5b, 0xdd, 0x3b, 0x63, 0x8a, 0xde,
	0xf3, 0x22, 0xaa, 0xe8, 0x3c, 0x57, 0x54, 0xe5, 0xee, 0x32, 0x93, 0x4a, 0x92, 0x3d, 0x8d, 0x76,
	0x11, 0xed, 0x6a, 0xb4, 0x6b, 0xd0, 0x93, 0xdb, 0x86, 0x8a, 0x2e, 0x13, 0x8f, 0x0a, 0x21, 0x15,
	0x55, 0x89, 0x14, 0xc6, 0x75, 0xfa, 0x4b, 0x07, 0xba, 0x0f, 0xa9, 0xa2, 0xc7, 0x25, 0x1d, 0x79,
	0x0a, 0x83, 0x05, 0x97, 0x54, 0x7d, 0xf0, 0x9e, 0xe6, 0xbf, 0xd5, 0xbe, 0xe3, 0xcc, 0x7a, 0xf7,
	0xdf, 0x70, 0x1b, 0x02, 0xb8, 0x47, 0xda, 0x03, 0x19, 0x1e, 0xbf, 0x14, 0xf4, 0x17, 0x96, 0x4c,
	0x9e, 0x40, 0x3f, 0x57, 0x59, 0x22, 0x62, 0x43, 0x78, 0x0d, 0x09, 0x67, 0x8d, 0x84, 0xc7, 0xe8,
	0x50, 0xf1, 0xf5, 0xf2, 0x8d, 0x48, 0x4e, 0x60, 0xa4, 0x92, 0x94, 0xe5, 0x8a, 0xa6, 0x4b, 0xc3,
	0xd8, 0x41, 0xc6, 0x37, 0x1b, 0x19, 0xbf, 0xae, 0x7c, 0x2a, 0xd2, 0xa1, 0xaa, 0x69, 0xc8, 0xe7,
	0xd0, 0xa3, 0x59, 0x46, 0xcf, 0x0d, 0xe7, 0x75, 0xe4, 0x7c, 0xbd, 0x91, 0x73, 0xbf, 0xc4, 0x57,
	0x7c, 0x40, 0xd7, 0x92, 0x29, 0xb9, 0x08, 0x95, 0x21, 0xdb, 0xd9, 0xae, 0xe4, 0x22, 0x54, 0x76,
	0xc9, 0x95, 0x48, 0x8e, 0x61, 0x18, 0x52, 0xc5, 0x62, 0x99, 0x55, 0xd9, 0xdd, 0x40, 0xc2, 0xbb,
	0x8d, 0x84, 0x87, 0xc6, 0xa5, 0xa2, 0x1c, 0x84, 0xb6, 0x82, 0xbc, 0x03, 0x37, 0xa3, 0x24, 0x57,
	0x89, 0x08, 0xd5, 0x7c, 0x45, 0x79, 0xc1, 0xe6, 0xa1, 0x2c, 0x84, 0xba, 0xe5, 0xdc, 0x71, 0x66,
	0xed, 0x80, 0x54, 0xb6, 0x93, 0xd2, 0x74, 0x58, 0x5a, 0xc8, 0x0c, 0xc6, 0xa2, 0xe0, 0xbc, 0x86,
	0x6e, 0x21, 0x7a, 0x58, 0xea, 0x2d, 0xe4, 0x5d, 0xd8, 0x5d, 0x51, 0x9e, 0x44, 0x35, 0x68, 0x17,
	0xa1, 0x23, 0x34, 0x6c, 0xb0, 0x07, 0x3b, 0xd0, 0xc1, 0x9a, 0xa6, 0x2f, 0x5a, 0xd0, 0xb7, 0x17,
	0x89, 0x10, 0xb8, 0x96, 0x32, 0x2a, 0x30, 0x23, 0x27, 0xc0, 0x6f, 0xf2, 0x36, 0x90, 0x5c, 0x51,
	0x11, 0xd1, 0x2c, 0x9a, 0x47, 0x6c, 0x95, 0xe0, 0x26, 0x63, 0x16, 0x4e, 0xb0, 0x5b, 0x59, 0x1e,
	0x56, 0x06, 0x72, 0x1b, 0xba, 0x3f, 0x16, 0x54, 0xa8, 0x84, 0xb3, 0x72, 0x93, 0xdb, 0x33, 0x27,
	0xd8, 0x28, 0xc8, 0x02, 0x76, 0x9f, 0x25, 0xb9, 0x92, 0x71, 0x46, 0xd3, 0xf9, 0x59, 0x11, 0x3e,
	0x67, 0xb8, 0x9e, 0xed, 0x59, 0xef, 0xfe, 0x87, 0x5b, 0xef, 0xbb, 0xfb, 0xb8, 0xa2, 0x38, 0x40,
	0x86, 0x60, 0xfc, 0xac, 0xae, 0xc8, 0x27, 0x5f, 0xc0, 0xe8, 0x02, 0x88, 0x8c, 0xa1, 0x9d, 0x26,
	0x55, 0x69, 0xe5, 0x27, 0x6a, 0xe8, 0x4f, 0xa6, 0x94, 0xf2, 0x93, 0xdc, 0x84, 0x8e, 0xee, 0x5c,
	0x1b, 0x3b, 0xa7, 0x85, 0xe9, 0x6f, 0x0e, 0xf4, 0xac, 0xe7, 0x41, 0x28, 0xec, 0x2a, 0xb9, 0x9c,
	0x17, 0x22, 0xc1, 0x32, 0xf4, 0x7e, 0x38, 0x58, 0xc4, 0xfb, 0xdb, 0xbe, 0x31, 0xf7, 0x1b, 0xed,
	0x8d, 0x42, 0x30, 0x52, 0x72, 0x69, 0x2b, 0x26, 0x3e, 0xf4, 0x6d, 0xb9, 0x4c, 0x0c, 0x07, 0x8b,
	0xe9, 0x77, 0x03, 0x2d, 0x6c, 0xd2, 0x6d, 0xd9, 0xe9, 0xbe, 0x68, 0xc3, 0xb0, 0xfe, 0xf6, 0x08,
	0x83, 0x61, 0x9c, 0x51, 0x51, 0x70, 0x9a, 0xd5, 0xd2, 0xfd, 0xe4, 0x0a, 0x0f, 0xd8, 0x7d, 0x64,
	0x18, 0x50, 0xfa, 0x54, 0xa8, 0xec, 0x3c, 0x18, 0xc4, 0xb6, 0x6e, 0xf2, 0xbb, 0x03, 0x83, 0x1a,
	0x8a, 0xfc, 0x00, 0x3b, 0xd5, 0x94, 0x75, 0xc4, 0xa3, 0xff, 0x1d, 0xd1, 0x35, 0xb3, 0xd5, 0x91,
	0x2b, 0xda, 0xb2, 0x53, 0xb6, 0xa1, 0x1c, 0xea, 0x73, 0x76, 0x8e, 0x7d, 0xea, 0x04, 0xe5, 0xe7,
	0xa6, 0x77, 0xa6, 0x4b, 0x28, 0xf8, 0xad, 0x07, 0xce, 0xe4, 0x67, 0x20, 0xff, 0x2e, 0xca, 0x66,
	0xe8, 0x6a, 0x86, 0xaf, 0x6c, 0x86, 0xff, 0xda, 0xd4, 0xa6, 0x1a, 0xac, 0xe0, 0xd3, 0x6f, 0x01,
	0x36, 0xd7, 0x8c, 0x7c, 0x06, 0xfd, 0x94, 0xa5, 0x67, 0xac, 0x9a, 0x8f, 0x8e, 0xf4, 0x5a, 0x63,
	0xa4, 0xf5, 0x4f, 0x48, 0xd0, 0xd3, 0xbe, 0x28, 0x4c, 0xff, 0xd2, 0xeb, 0xba, 0xbe, 0x65, 0xdf,
	0x41, 0x6f, 0x91, 0x30, 0x1e, 0xd5, 0x26, 0xff, 0x60, 0xdb, 0xcb, 0xe8, 0x1e, 0x95, 0xbe, 0xd6,
	0xcc, 0x61, 0xb1, 0x56, 0x4c, 0x18, 0x8c, 0x2e, 0x98, 0x2f, 0xe9, 0xde, 0xc7, 0xf5, 0xee, 0x6d,
	0x5b, 0x93, 0xd5, 0xaa, 0x3f, 0x1c, 0x18, 0xd4, 0x6e, 0x2b, 0xe1, 0x40, 0xca, 0x27, 0x78, 0xe1,
	0x46, 0x6f, 0xb3, 0xd4, 0x35, 0x1e, 0xf7, 0x38, 0x11, 0x31, 0x67, 0x35, 0x5d, 0x30, 0x56, 0x72,
	0x59, 0xd3, 0x4c, 0xf6, 0xe1, 0xe5, 0x4b, 0x80, 0x57, 0x7a, 0x94, 0x1e, 0x8c, 0x0f, 0x65, 0x96,
	0x31, 0x8e, 0x57, 0x52, 0xfb, 0xef, 0x41, 0x37, 0xcc, 0x68, 0xca, 0xb2, 0x7c, 0xbe, 0x32, 0x77,
	0xe9, 0x86, 0x51, 0x9c, 0x1c, 0xfc, 0xea, 0xc0, 0xab, 0xa1, 0x4c, 0x9b, 0x6a, 0x79, 0xea, 0x7c,
	0xbf, 0x6f, 0xcc, 0xb1, 0xe4, 0x54, 0xc4, 0xae, 0xcc, 0x62, 0x2f, 0x66, 0x02, 0xff, 0x65, 0x78,
	0xda, 0x44, 0x97, 0x49, 0x7e, 0xe9, 0x3f, 0x9a, 0x8f, 0xb4, 0xf8, 0x67, 0x6b, 0xef, 0x11, 0x02,
	0x4f, 0x0f, 0x4b, 0xd0, 0xe9, 0x7e, 0xa1, 0xe4, 0x13, 0x7e, 0x7a, 0xa2, 0x41, 0x7f, 0xb7, 0x5e,
	0xd1, 0x56, 0xdf, 0x47, 0xb3, 0xef, 0xa3, 0xfd, 0x4b, 0xdf, 0x37, 0x80, 0xb3, 0xeb, 0x18, 0xec,
	0xdd, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff, 0x25, 0x00, 0xaf, 0x85, 0x3d, 0x09, 0x00, 0x00,
}