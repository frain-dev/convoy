import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, Input, OnDestroy, OnInit } from '@angular/core';
import { AdminService } from '../admin/admin.service';
import type { QueueStore, QueueStoreSnapshot } from './queue-monitoring-stores.component';

interface Operation { id: string; state: string; revision: number; purpose: string; updated_at: string; }
interface DrainStatus {
 operation: Operation | null;
 configuration_revision: string;
 paused_queues: string[] | null;
 snapshot: QueueStoreSnapshot | null;
 workers: Array<{id: string; store_id: string; operation_id: string; revision: number; state: string; observed_at: string}>;
 unsettled: number;
 blockers: string[];
 actions: string[];
}
@Component({selector: 'convoy-queue-drain', imports: [CommonModule], templateUrl: './queue-drain.component.html'})
export class QueueDrainComponent implements OnInit, OnDestroy {
 @Input({required:true}) store!: QueueStore;
 status?: DrainStatus;
 stale = false;
 busy = false;
 error = '';
 selected = '';
 resumePaused=false;
 private requestKey = '';
 private destroyed = false;
 private refreshing = false;
 private timer?: ReturnType<typeof setTimeout>;
 constructor(private readonly admin: AdminService, private readonly cdr: ChangeDetectorRef) {}
 ngOnInit(): void { void this.refresh(); }
 ngOnDestroy(): void {this.destroyed=true;clearTimeout(this.timer);}
 async refresh(): Promise<void> {
  if(this.destroyed || this.refreshing)return;
  this.refreshing=true;clearTimeout(this.timer);
  try {
   const response=await this.admin.getQueueDrain(this.store.id);
   if(this.destroyed)return;
   this.status=response.data as DrainStatus;
   this.stale=false;
  }catch{if(!this.destroyed)this.stale=true;}
  finally{this.refreshing=false;if(!this.destroyed){this.cdr.markForCheck();this.timer=setTimeout(()=>void this.refresh(),3000);}}
 }
 get waitingCount(): number {return this.status?.snapshot?.queues.reduce((n,q)=>n+q.scheduled+q.retry+q.future+q.aggregating,0) ?? 0;}
 get blockedCount(): number {return this.status?.snapshot?.queues.reduce((n,q)=>n+q.archived+q.unknown,0) ?? 0;}
 get pendingCount(): number {return this.status?.snapshot?.queues.reduce((n,q)=>n+q.pending,0) ?? 0;}
 get processingCount(): number {return this.status?.snapshot?.queues.reduce((n,q)=>n+q.processing,0) ?? 0;}
 get needsReviewCount(): number {return this.blockedCount+(this.status?.unsettled ?? 0);}
 label(action: string): string {
  const labels: Record<string,string>={quiesce:'Pause processing',drain:'Drain queue',drain_previous:'Drain previous queue',stop:'Stop drain',resume_drain:'Continue drain',resume_traffic:'Resume traffic'};
  return labels[action] || action;
 }
 stateLabel(state: string): string {
  const labels: Record<string,string>={resumed:'Running',quiesced:'Paused',draining:'Draining',drained:'Drained',stopped:'Stopped',fencing:'Pausing'};
  return labels[state] || state.replaceAll('_',' ').replace(/^./,c=>c.toUpperCase());
 }
 select(action: string): void {this.resumePaused=false;this.selected=action;this.error='';this.requestKey=crypto.randomUUID();}
 cancel(): void {if(!this.busy){this.selected='';this.requestKey='';}}
 async confirm(): Promise<void> {
  if(this.busy||this.stale||!this.status||!this.selected||!this.status.actions.includes(this.selected))return;
  if(this.selected!=="quiesce" && ["drain","drain_previous"].includes(this.selected) && this.status.paused_queues?.length && !this.resumePaused)return;
  this.busy=true;this.error='';
  try{
   if(['quiesce','drain','drain_previous'].includes(this.selected)){
    await this.admin.beginQueueDrain(this.store.id,{purpose:this.selected,configuration_revision:this.status.configuration_revision,idempotency_key:this.requestKey,resume_paused:this.resumePaused});
   }else if(this.status.operation){
    await this.admin.commandQueueDrain(this.store.id,this.status.operation.id,{revision:this.status.operation.revision,command:this.selected});
   }
   this.selected='';this.requestKey='';await this.refresh();
  }catch{this.error='The command was not confirmed. Refresh to check its status before retrying.';await this.refresh();}
  finally{this.busy=false;if(!this.destroyed)this.cdr.markForCheck();}
 }
}
