import { ComponentFixture, TestBed, fakeAsync, flushMicrotasks } from '@angular/core/testing';
import { AdminService } from '../admin/admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { QueueMaintenanceComponent } from './queue-maintenance.component';

describe('QueueMaintenanceComponent access', () => {
 let fixture: ComponentFixture<QueueMaintenanceComponent>;
 let role: jasmine.Spy, license: jasmine.Spy, stores: jasmine.Spy;
 beforeEach(async () => {
  role = jasmine.createSpy().and.resolveTo('INSTANCE_ADMIN');
  license = jasmine.createSpy().and.returnValue(true);
  stores = jasmine.createSpy().and.resolveTo({ data: [] });
  await TestBed.configureTestingModule({ imports: [QueueMaintenanceComponent], providers: [
   {provide: RbacService, useValue: {getUserRole: role}},
   {provide: LicensesService, useValue: {loadAllLicenses: async () => {}, hasInstanceLicense: license}},
   {provide: AdminService, useValue: {getQueueStores: stores}}
  ] }).compileComponents();
  fixture = TestBed.createComponent(QueueMaintenanceComponent);
 });
 afterEach(() => fixture.destroy());
 it('does not fetch operations before access resolves', () => {
  role.and.returnValue(new Promise(() => {}));
  fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('Checking access');
  expect(stores).not.toHaveBeenCalled();
 });
 for (const failure of ['role', 'license', 'unavailable']) {
  it('withholds controls when access fails: ' + failure, fakeAsync(() => {
   if (failure === 'role') role.and.resolveTo('ORG_ADMIN');
   if (failure === 'license') license.and.returnValue(false);
   if (failure === 'unavailable') role.and.rejectWith(new Error('offline'));
   fixture.detectChanges(); flushMicrotasks(); fixture.detectChanges();
   expect(fixture.nativeElement.textContent).toContain('requires instance administrator');
   expect(stores).not.toHaveBeenCalled();
  }));
 }
 it('opens the maintenance view only for an authorized administrator', fakeAsync(() => {
  fixture.detectChanges(); flushMicrotasks(); fixture.detectChanges();
  expect(stores).toHaveBeenCalledTimes(1);
  expect(fixture.nativeElement.querySelector('convoy-queue-monitoring-stores')).not.toBeNull();
 }));
});
