import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { PermissionDirective } from './permission.directive';
import { RbacService } from 'src/app/services/rbac/rbac.service';

@Component({
    imports: [PermissionDirective],
    template: `<button convoy-permission="Endpoints|VIEW">action</button>`
})
class PermissionHostComponent {}

describe('PermissionDirective', () => {
	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [PermissionHostComponent],
			providers: [
				{ provide: RbacService, useValue: { userPermission: async () => ['Endpoints|VIEW'] as const } },
				{ provide: ActivatedRoute, useValue: { snapshot: { queryParams: {} } } }
			]
		}).compileComponents();
	});

	it('should create', () => {
		const fixture = TestBed.createComponent(PermissionHostComponent);
		fixture.detectChanges();
		expect(fixture).toBeTruthy();
	});
});
