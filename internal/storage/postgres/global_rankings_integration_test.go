//go:build integration

package postgres

import (
 "fmt"
 "reflect"
 "testing"
 "time"
 "github.com/malbs/UnoGoBot/internal/groups"
 "github.com/malbs/UnoGoBot/internal/ranking"
)

func TestGlobalMonthlyRankings(t *testing.T){
 s:=prepareRanking(t,groups.Updated);ctx:=t.Context();at:=time.Date(2026,9,15,12,0,0,0,ranking.RankingLocation)
 record:=func(chat int64,system groups.RankingSystem,id string,at time.Time,swap bool){t.Helper();if _,err:=s.GetOrCreateGroupConfig(ctx,chat);err!=nil{t.Fatal(err)};if _,err:=s.pool.Exec(ctx,`UPDATE group_configs SET ranking_system=$1,title=$2 WHERE chat_id=$3`,system,fmt.Sprintf("Grupo %d",chat),chat);err!=nil{t.Fatal(err)}
 r:=eligibleResult(t,system,3,1,"completed");r.ChatID=chat;r.GameID=id;r.StartedAt=at.Add(-time.Hour);r.FinishedAt=at;r.Players[0].DisplayName="Freddy";r.Players[1].DisplayName="Mezi";r.Players[2].DisplayName="João 🃏";if swap{r.Players[0].UserID,r.Players[1].UserID=2,1;r.Players[0].DisplayName,r.Players[1].DisplayName="Mezi novo","Freddy novo"};if _,err:=s.RecordCompletedGame(ctx,r);err!=nil{t.Fatal(err)};if _,err:=s.RecordCompletedGame(ctx,r);err!=nil{t.Fatal(err)}}
 record(42,groups.Updated,"A",at,false);record(42,groups.Updated,"B",at.Add(time.Hour),true)
 record(43,groups.Updated,"C",at.Add(2*time.Hour),false)
 record(44,groups.Legacy,"legacy",at,false)
 record(45,groups.Updated,"old",at.AddDate(0,-1,0),false)
 req:=ranking.GlobalRequest{Kind:"groups",System:groups.Updated,Month:ranking.MonthStart(at),Limit:1}
 first,err:=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)}
 if len(first.Rows)!=1||!first.More||first.Rows[0].ID!=42||first.Rows[0].Score!=3000{t.Fatalf("%+v",first)}
 key:=first.Rows[0].Key();req.After=&key;second,err:=s.ReadGlobalRanking(ctx,req);if err!=nil||second.More||len(second.Rows)!=1||second.Rows[0].ID!=43||second.Rows[0].Position!=2{t.Fatal(second,err)}
 req.After=nil;req.Limit=50;req.Kind="players";players,err:=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)}
 if len(players.Rows)!=3||players.Rows[0].Score!=2500||players.Rows[0].Name!="Freddy"||players.Rows[2].Score!=0{t.Fatalf("%+v",players)}
 for _,r:=range players.Rows{if r.ID==4{t.Fatal("abandoner leaked")}}
 key=players.Rows[0].Key();req.After=&key;tail,err:=s.ReadGlobalRanking(ctx,req);if err!=nil||len(tail.Rows)!=2||tail.Rows[0].Position!=2{t.Fatal(tail,err)}
 req.After=nil;req.Kind="detail";req.GroupID=42;detail,err:=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)}
 telegram,err:=s.ListGroupRanking(ctx,42,at);if err!=nil{t.Fatal(err)}
 if detail.Group.Score!=3000||len(detail.Rows)!=len(telegram.Entries){t.Fatal(detail)}
 for i,row:=range detail.Rows{if row.ID!=telegram.Entries[i].UserID||row.Score!=telegram.Entries[i].Score{t.Fatal("diverged",detail,telegram)}}
 if detail.Rows[0].ID!=2||detail.Rows[1].ID!=1{t.Fatal("placement tie",detail)}
 key=detail.Rows[0].Key();req.After=&key;tail,err=s.ReadGlobalRanking(ctx,req);if err!=nil||len(tail.Rows)!=2||tail.Rows[0].ID!=1{t.Fatal(tail,err)}
 req.After=nil;req.Kind="players";req.System=groups.Legacy;legacy,err:=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)}
 if len(legacy.Rows)!=3||legacy.Rows[0].Score!=100{t.Fatal(legacy)}
 req.Kind="groups";legacy,err=s.ReadGlobalRanking(ctx,req);if err!=nil||len(legacy.Rows)!=1||legacy.Rows[0].ID!=44{t.Fatal(legacy,err)}
 // An uncommitted aggregate change must not leak into another connection's read.
 req.System=groups.Updated;tx,err:=s.pool.Begin(ctx);if err!=nil{t.Fatal(err)};defer tx.Rollback(ctx)
 if _,err=tx.Exec(ctx,`UPDATE player_group_monthly_stats SET score_units=999999 WHERE chat_id=42`);err!=nil{t.Fatal(err)}
 stable,err:=s.ReadGlobalRanking(ctx,req);if err!=nil||stable.Rows[0].Score!=3000{t.Fatal(stable,err)}
}

func TestGlobalStableTieOrdering(t *testing.T){
 s:=prepareRanking(t,groups.Updated);ctx:=t.Context();at:=ranking.MonthStart(time.Now()).Add(12*time.Hour)
 for _,id:=range []int64{42,43,44}{s.GetOrCreateGroupConfig(ctx,id);r:=eligibleResult(t,groups.Updated,2,0,"completed");r.GameID=fmt.Sprint(id);r.ChatID=id;r.StartedAt=at.Add(-time.Hour);r.FinishedAt=at;r.Players[0].UserID=id;r.Players[1].UserID=id+100;r.Players[0].DisplayName="Mesmo";r.Players[1].DisplayName="Zero";s.pool.Exec(ctx,`UPDATE group_configs SET ranking_system='updated',title='Mesmo' WHERE chat_id=$1`,id);if _,err:=s.RecordCompletedGame(ctx,r);err!=nil{t.Fatal(err)}}
 req:=ranking.GlobalRequest{Kind:"groups",System:groups.Updated,Month:ranking.MonthStart(at),Limit:50};page,err:=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)}
 ids:=[]int64{};for _,row:=range page.Rows{ids=append(ids,row.ID)};if !reflect.DeepEqual(ids,[]int64{42,43,44}){t.Fatal(ids)}
 req.Kind="players";page,err=s.ReadGlobalRanking(ctx,req);if err!=nil{t.Fatal(err)};for i:=0;i<3;i++{if page.Rows[i].ID!=int64(42+i)||page.Rows[i].Position!=int64(i+1){t.Fatal(page)}}
}
