import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';

import { PrivateRoutingModule } from './private-routing.module';
import { PrivateComponent } from './private.component';
import { ButtonComponent } from '../components/button/button.component';
import { BadgeComponent } from '../components/badge/badge.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { GithubStarComponent } from '../components/github-star/github-star.component';
import { VerifyEmailComponent } from './components/verify-email/verify-email.component';
import { LoaderModule } from './components/loader/loader.module';
import { ReactiveFormsModule } from '@angular/forms';
import { InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent } from '../components/input/input.component';
import { DialogDirective } from '../components/modal/modal.component';

@NgModule({
	declarations: [PrivateComponent],
	imports: [
		CommonModule,
		PrivateRoutingModule,
		DropdownComponent,
		ButtonComponent,
		BadgeComponent,
		GithubStarComponent,
		DropdownOptionDirective,
		VerifyEmailComponent,
		LoaderModule,
		ButtonComponent,
		ReactiveFormsModule,
		InputFieldDirective,
		InputErrorComponent,
		InputDirective,
		LabelComponent,
        DialogDirective
	]
})
export class PrivateModule {}
