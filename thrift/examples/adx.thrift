namespace php Dirpc.SDK.Adx
namespace go Dirpc.SDK.Adx

struct TimesegsItem {
    1:required i64        end_time
    2:required i64        start_time
}

struct AdxMgetReq {
    1: optional string resource_names
    2: optional i32    channel_id
    3: optional i32    coop_mode
    4: optional i32    origin_id
}

struct AdxMgetRespDataForDriApi {
    1: required i64            activity_id
    2: required string         image
    3: required string         link
}

struct AdxMgetRespDataForKFDriApi {
    1: required i64            activity_id
    2: required string         image
    3: required string         link
}

struct AdxMgetRespDataListForDriApi {
    1: optional list<AdxMgetRespDataForDriApi> dididri_profile_banners
    2: optional list<AdxMgetRespDataForKFDriApi> d_home_banner
}

struct AdxMgetRespForDriApi {
    1: required i32                                errno 
    2: required string                             errmsg 
    3: required AdxMgetRespDataListForDriApi       data 
}

struct Attach {
    1: optional string link
    2: optional string sign
    3: optional string size
}

struct AdxMgetRespDataFields {
    1: required i64                activity_id
    2: optional string             image
}

typedef list<AdxMgetRespDataFields> AdxMgetRespDataList

struct AdxMgetResp {
    1: required i32                             errno 
    2: required string                          errmsg 
    3: required map<string,list<AdxMgetRespDataFields>> data 
}

struct AdxGetPosReq {
    1: required string resource_names
    2: optional i32    channel_id
}

struct Tag {
    1: required string type
    2: required string value
}

struct GetPosMaterial {
    1: required string type
    2: required string log_data // (php.type="array",go.type="interface{}")
    4: required list<string> imp_tracks
    5: required list<string> click_tracks
}

struct AdxGetPosDetailResp {
    1: required string title
    2: required string sub_title
    3: required string icon
    4: optional GetPosMaterial big_image
    5: required list<GetPosMaterial> material_list
}

struct AdxGetPosResp {
    1: required i32                             errno 
    2: required string                          errmsg 
    3: required map<string, AdxGetPosDetailResp> data 
}

struct AdResp{
    1: required i32    errno
    2: required string errmsg
    3: required string data  // (php.type="array",go.type="interface{}")
}

service AdxService {

    AdxMgetRespForApi MgetForDriApi(1: AdxMgetReq req) (
        path="/resapi/activity/mget"
        httpMethod="post"
        contentType="form"
    )

    AdxMgetResp Mget(1: AdxMgetReq req) (
        path="/resapi/activity/mget"
        httpMethod="post"
        contentType="form"
    )

    AdxGetResp GetPos(1: AdxGetPosReq req) (
        path="/resapi/activity/getpoi"
        httpMethod="post"
        contentType="form"
    )

    AdResp AdGet(1: AdxMgetReq req) (
        path="/resapi/activity/mgetad"
        httpMethod="post"
        contentType="form"
    )
} (
    version="1.1.23"
    servName="hello/world"
    servType="http"
    timeoutMsec="200"
    connectTimeoutMsec="100"
)
