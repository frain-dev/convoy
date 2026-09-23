import { ComponentFixture, TestBed } from '@angular/core/testing';
import { AdminService } from '../admin/admin.service';
import { QueueDrainComponent } from './queue-drain.component';

describe('QueueDrainComponent', () => {
 let fixture: ComponentFixture<QueueDrainComponent>;
 let api: {getQueueDrain: jasmine.Spy; beginQueueDrain: jasmine.Spy; commandQueueDrain: jasmine.Spy};
 const status = () => ({operation:null, configuration_revision:'cfg', snapshot:{observed_at:'2026-09-22T12:00:00Z',queues:[]}, workers:[], unsettled:0, blockers:[], actions:['drain'], paused_queues:['events']});
 beforeEach(async()=>{
  api={getQueueDrain:jasmine.createSpy().and.resolveTo({data:status()}),beginQueueDrain:jasmine.createSpy().and.resolveTo({}),commandQueueDrain:jasmine.createSpy().and.resolveTo({})};
  await TestBed.configureTestingModule({imports:[QueueDrainComponent],providers:[{provide:AdminService,useValue:api}]}).compileComponents();
  fixture=TestBed.createComponent(QueueDrainComponent);
  fixture.componentInstance.store={id:'active',provider:'redis',role:'active',connection:'connected',visible:true,visibility_reasons:[],actions:['review'],snapshot:null};
  fixture.detectChanges();await fixture.whenStable();fixture.detectChanges();
 });
 afterEach(()=>fixture.destroy());
 function confirmButton():HTMLButtonElement {return Array.from(fixture.nativeElement.querySelectorAll('button')).find((b:any)=>b.textContent.includes('Confirm')) as HTMLButtonElement;}
 it('requires paused consent through a visible checkbox',async()=>{
  fixture.nativeElement.querySelector('button').click();fixture.detectChanges();expect(confirmButton().disabled).toBeTrue();
  const checkbox:HTMLInputElement=fixture.nativeElement.querySelector('input[type=checkbox]');
  expect(checkbox.getBoundingClientRect().width).toBeGreaterThan(0);
  checkbox.click();fixture.detectChanges();expect(confirmButton().disabled).toBeFalse();
  confirmButton().click();await fixture.whenStable();
  expect(api.beginQueueDrain).toHaveBeenCalledWith('active',jasmine.objectContaining({purpose:'drain',resume_paused:true,configuration_revision:'cfg'}));
 });
 it('does not begin from opening or selecting',()=>{fixture.componentInstance.select('drain');expect(api.beginQueueDrain).not.toHaveBeenCalled();});
 it('marks stale observations and disables commands on refresh failure',async()=>{
  api.getQueueDrain.and.rejectWith(new Error('offline'));await fixture.componentInstance.refresh();fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('stale');expect(fixture.nativeElement.querySelector('button').disabled).toBeTrue();
  await fixture.componentInstance.confirm();expect(api.beginQueueDrain).not.toHaveBeenCalled();
 });
 it('reuses the request key after an ambiguous begin failure',async()=>{
  const c=fixture.componentInstance;c.select('drain');c.resumePaused=true;api.beginQueueDrain.and.rejectWith(new Error('timeout'));
  await c.confirm();await c.confirm();expect(api.beginQueueDrain.calls.argsFor(0)[1].idempotency_key).toBe(api.beginQueueDrain.calls.argsFor(1)[1].idempotency_key);
 });
 it('resets paused consent for a new confirmation',()=>{const c=fixture.componentInstance;c.resumePaused=true;c.select('drain');expect(c.resumePaused).toBeFalse();});
 it('uses the durable revision for resume',async()=>{
  api.getQueueDrain.and.resolveTo({data:{...status(),operation:{id:'op',state:'drained',revision:7,purpose:'drain',updated_at:'2026-09-22T12:00:00Z'},actions:['resume_traffic']}});
  const c=fixture.componentInstance;await c.refresh();c.select('resume_traffic');await c.confirm();
  expect(api.commandQueueDrain).toHaveBeenCalledWith('active','op',{revision:7,command:'resume_traffic'});
 });
});
