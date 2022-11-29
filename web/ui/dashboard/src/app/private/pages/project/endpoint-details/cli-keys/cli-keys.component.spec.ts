import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import {} from 'src/app/components/input/input.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';

import { CliKeysComponent } from './cli-keys.component';

describe('CliKeysComponent', () => {
	let component: CliKeysComponent;
	let fixture: ComponentFixture<CliKeysComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [CliKeysComponent, CardComponent, ModalComponent, ButtonComponent, SkeletonLoaderComponent, EmptyStateComponent, DeleteModalComponent, SelectComponent, RouterTestingModule, ReactiveFormsModule]
		}).compileComponents();

		fixture = TestBed.createComponent(CliKeysComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
