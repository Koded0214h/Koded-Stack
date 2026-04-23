from rest_framework import viewsets, status, generics
from rest_framework.decorators import action
from rest_framework.response import Response
from rest_framework.views import APIView
from django.utils import timezone
from datetime import timedelta
from django.db.models import Count
from .models import WaitlistUser
from .serializers import (
    WaitlistUserSerializer, 
    WaitlistCreateSerializer,
    WaitlistStatsSerializer
)

class WaitlistViewSet(viewsets.ModelViewSet):
    queryset = WaitlistUser.objects.all()
    
    def get_serializer_class(self):
        if self.action == 'create':
            return WaitlistCreateSerializer
        return WaitlistUserSerializer
    
    def create(self, request, *args, **kwargs):
        serializer = self.get_serializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        
        # Get client info
        ip_address = request.META.get('REMOTE_ADDR')
        user_agent = request.META.get('HTTP_USER_AGENT', '')
        
        # Create user
        user = WaitlistUser.objects.create(
            **serializer.validated_data,
            ip_address=ip_address,
            user_agent=user_agent
        )
        
        return Response(
            WaitlistUserSerializer(user).data,
            status=status.HTTP_201_CREATED
        )
    
    @action(detail=False, methods=['get'])
    def stats(self, request):
        # Total users
        total = WaitlistUser.objects.count()
        
        # Confirmed users
        confirmed = WaitlistUser.objects.filter(is_confirmed=True).count()
        
        # Last 24 hours
        yesterday = timezone.now() - timedelta(days=1)
        last_24h = WaitlistUser.objects.filter(created_at__gte=yesterday).count()
        
        # By source
        by_source = dict(WaitlistUser.objects.values_list('referral_source')
            .annotate(count=Count('id'))
            .values_list('referral_source', 'count')
        )
        
        # Estimated wait (7 days per 100 users)
        estimated_days = max(7, (total // 100) * 7)
        
        stats = {
            'total_users': total,
            'confirmed_users': confirmed,
            'last_24_hours': last_24h,
            'by_source': by_source,
            'estimated_wait_days': estimated_days
        }
        
        serializer = WaitlistStatsSerializer(stats)
        return Response(serializer.data)
    
    @action(detail=False, methods=['get'])
    def check(self, request):
        email = request.query_params.get('email')
        if not email:
            return Response(
                {'error': 'Email parameter is required'},
                status=status.HTTP_400_BAD_REQUEST
            )
        
        try:
            user = WaitlistUser.objects.get(email=email)
            return Response({
                'is_on_waitlist': True,
                'position': user.position,
                'joined_at': user.created_at,
                'total_users': WaitlistUser.objects.count()
            })
        except WaitlistUser.DoesNotExist:
            return Response({
                'is_on_waitlist': False,
                'message': 'Email not found on waitlist'
            })

class TotalUsersView(APIView):
    def get(self, request):
        total = WaitlistUser.objects.count()
        return Response({'total_users': total})