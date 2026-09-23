import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, OnDestroy, OnInit } from '@angular/core';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { QueueMonitoringStoresComponent } from './queue-monitoring-stores.component';

@Component({
 selector: 'convoy-queue-maintenance',
 imports: [CommonModule, QueueMonitoringStoresComponent],
 template: `
  <h2 class="font-medium text-[18px] mb-16px">Queue maintenance</h2>
  @if (loading) { <p role="status" class="text-[14px] text-new.text-secondary">Checking access…</p> }
  @else if (!allowed) { <p role="status" class="text-[14px] text-new.text-secondary">Queue maintenance requires instance administrator access and a queue monitoring license.</p> }
  @else { <convoy-queue-monitoring-stores [maintenance]="true" /> }
 `
})
export class QueueMaintenanceComponent implements OnInit, OnDestroy {
 private destroyed = false;
 loading = true;
 allowed = false;
 constructor(private readonly licenses: LicensesService, private readonly rbac: RbacService, private readonly cdr: ChangeDetectorRef) {}
 ngOnDestroy(): void { this.destroyed = true; }
 async ngOnInit(): Promise<void> {
  try {
   if (await this.rbac.getUserRole() !== 'INSTANCE_ADMIN') return;
   await this.licenses.loadAllLicenses();
   this.allowed = this.licenses.hasInstanceLicense('AsynqMonitoring');
  } catch { this.allowed = false; }
  finally { this.loading = false; if (!this.destroyed) this.cdr.markForCheck(); }
 }
}
