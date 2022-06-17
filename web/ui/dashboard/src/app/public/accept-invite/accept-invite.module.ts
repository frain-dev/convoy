import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AcceptInviteComponent } from './accept-invite.component';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

const routes: Routes = [{ path: '', component: AcceptInviteComponent }];

@NgModule({
	declarations: [AcceptInviteComponent],
	imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterModule.forChild(routes), LoaderModule]
})
export class AcceptInviteModule {}
